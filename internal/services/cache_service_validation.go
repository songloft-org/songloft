package services

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"songloft/internal/models"
	"songloft/internal/services/source"
)

// Only file metadata is memoized; current thresholds and expected duration are
// evaluated on every hit. Bound memory independently of the disk cache size.
const maxValidatedFiles = 1024

type cachedAudioInfo struct {
	stat os.FileInfo
	info *AudioInfo
}

// SetSourceValidation shares the source fetcher's metadata extractor and health
// metrics. onInvalid starts the existing deduplicated background source fallback.
// Configure before serving requests.
func (c *CacheService) SetSourceValidation(extractor *MetadataExtractor, metrics *source.SourceMetrics, onInvalid func(*models.Song)) {
	c.validationExtractor = extractor
	c.validationMetrics = metrics
	c.onInvalidAudio = onInvalid
}

func (c *CacheService) sourceValidationOpts() source.ValidationOpts {
	opts := source.DefaultValidationOpts()
	if c.configService != nil {
		_ = c.configService.GetJSON("source_validation", &opts)
	}
	return opts
}

func (c *CacheService) validateAudioFile(ctx context.Context, song *models.Song, path string, memoize bool) error {
	if song == nil {
		return errors.New("song is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	opts := c.sourceValidationOpts()
	if !opts.Enabled {
		return nil
	}
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	var info *AudioInfo
	if memoize {
		c.validationMu.Lock()
		cached, ok := c.validatedFiles[path]
		c.validationMu.Unlock()
		if ok && os.SameFile(stat, cached.stat) && stat.Size() == cached.stat.Size() && stat.ModTime() == cached.stat.ModTime() {
			info = cached.info
		}
	}
	if info == nil {
		extractor := c.validationExtractor
		if extractor == nil {
			// Standalone CacheService users still get tag-based validation.
			// The application injects its extractor with configured ffprobe.
			extractor = NewMetadataExtractor(&MetadataConfig{})
		}
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		info, err = extractor.ProbeForValidation(probeCtx, path)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return &source.InvalidAudioError{Reason: source.ReasonProbeFailed, Expected: song.Duration}
		}
	}
	result := source.Validate(info, song.Duration, opts)
	if !result.Valid {
		return &source.InvalidAudioError{Reason: result.Reason, Expected: result.Expected, Actual: result.Actual}
	}
	if memoize {
		c.validationMu.Lock()
		if len(c.validatedFiles) >= maxValidatedFiles {
			clear(c.validatedFiles)
		}
		if c.validatedFiles == nil {
			c.validatedFiles = make(map[string]cachedAudioInfo)
		}
		c.validatedFiles[path] = cachedAudioInfo{stat: stat, info: info}
		c.validationMu.Unlock()
	}
	return nil
}

func (c *CacheService) reportInvalidAudio(song *models.Song, err error) {
	var invalid *source.InvalidAudioError
	if song == nil || !errors.As(err, &invalid) {
		return
	}
	slog.Warn("cache audio rejected", "songId", song.ID, "error", err)
	if c.validationMetrics != nil && song.IsPluginSourced() {
		c.validationMetrics.Record(source.Outcome{
			PluginEntryPath: song.PluginEntryPath,
			Result:          source.OutcomeValidationFail,
			Reason:          string(invalid.Reason),
			Timestamp:       time.Now(),
		})
	}
	if c.onInvalidAudio != nil && song.IsPluginSourced() {
		copy := *song
		c.onInvalidAudio(&copy)
	}
}

func (c *CacheService) findValidatedCache(ctx context.Context, song *models.Song) (string, bool) {
	path, ok := c.findCachedFileBySong(song)
	if !ok {
		return "", false
	}
	err := c.validateAudioFile(ctx, song, path, true)
	if err == nil {
		return path, true
	}
	var invalid *source.InvalidAudioError
	if errors.As(err, &invalid) {
		// Evict derivatives too: they may have been made from this same prompt.
		_ = c.EvictSong(song.ID, path)
		c.reportInvalidAudio(song, err)
	}
	return "", false
}
