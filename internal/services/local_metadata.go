package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"songloft/internal/models"
)

type IndexedLocalTrack struct {
	ID           int64  `json:"id"`
	FilePath     string `json:"file_path"`
	RelativePath string `json:"relative_path"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	Album        string `json:"album"`
}

func (s *SongService) LocalLibraryRoot() (string, error) {
	root, err := s.resolveMusicPath()
	if err != nil {
		return "", err
	}
	return filepath.Clean(root), nil
}

func (s *SongService) InspectLocalFile(ctx context.Context, id int64) (*LocalFileInfo, *LocalFileFailure) {
	root, err := s.LocalLibraryRoot()
	if err != nil {
		return nil, &LocalFileFailure{"error", err}
	}
	song, err := s.songs.GetByID(ctx, id)
	if err != nil {
		return nil, &LocalFileFailure{"changed", errors.New("the song or its path changed")}
	}
	info, failure := s.checkLocalSource(ctx, root, song)
	if failure != nil {
		return nil, failure
	}
	if failure := checkLocalContainer(song.FilePath); failure != nil {
		return nil, failure
	}
	digest, err := hashLocalSource(ctx, song.FilePath)
	if err != nil || !sameLocalFile(song.FilePath, info) {
		return nil, &LocalFileFailure{"changed", errors.New("the source file could not be read or changed")}
	}
	relative, err := filepath.Rel(root, song.FilePath)
	if err != nil {
		return nil, &LocalFileFailure{"changed", err}
	}
	protected := make([]string, 0)
	for _, field := range []string{"title", "artist", "album", "year", "track"} {
		if song.MetadataOverrideMask.Has(metadataFieldBits[field]) {
			protected = append(protected, field)
		}
	}
	sidecar, _ := FindSidecarLyricFile(song.FilePath)
	return &LocalFileInfo{
		Song: song, RelativePath: filepath.ToSlash(relative), Revision: localFileRevision(song, info, digest),
		Protected: protected, LookupLyrics: allowsLocalLyrics(song, sidecar), LookupCover: allowsLocalCover(song),
	}, nil
}

type LocalFileInfo struct {
	Song         *models.Song `json:"song"`
	RelativePath string       `json:"relative_path"`
	Revision     string       `json:"revision"`
	Protected    []string     `json:"protected"`
	LookupLyrics bool         `json:"lookup_lyrics"`
	LookupCover  bool         `json:"lookup_cover"`
}

func localFileRevision(song *models.Song, info os.FileInfo, digest string) string {
	return fmt.Sprintf("%s:%x:%x:%x:%x:%s", digest, info.Size(), info.ModTime().UnixNano(),
		song.MetadataEditedAt, song.MetadataOverrideMask, song.FilePath)
}

func allowsLocalLyrics(song *models.Song, sidecar string) bool {
	return sidecar == "" && !song.MetadataOverrideMask.Has(models.MetadataOverrideLyrics) &&
		(song.LyricSource == "" || song.LyricSource == models.LyricSourceScraped || song.LyricSource == models.LyricSourceCached)
}

func allowsLocalCover(song *models.Song) bool {
	return !song.MetadataOverrideMask.Has(models.MetadataOverridePicture) && song.CoverPath == "" && song.CoverURL == ""
}

// EnsureLocalFingerprint reuses a track's stored fingerprint or samples that
// track with the same Chromaprint extractor used by the scan service.
func (s *SongService) EnsureLocalFingerprint(ctx context.Context, id int64) (*models.Song, error) {
	lock := &s.metadataEditLocks[uint64(id)%uint64(len(s.metadataEditLocks))]
	lock.Lock()
	defer lock.Unlock()
	root, err := s.LocalLibraryRoot()
	if err != nil {
		return nil, err
	}
	song, err := s.songs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info, failure := s.checkLocalSource(ctx, root, song)
	if failure != nil {
		return nil, failure
	}
	if song.Fingerprint != "" || !IsChromaprintAvailable() {
		return song, nil
	}
	attemptedAt := time.Now().Unix()
	fp, duration, err := ExtractFingerprint(ctx, song.FilePath, 0, 0)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !sameLocalFile(song.FilePath, info) {
		return nil, errors.New("source file changed during fingerprint extraction")
	}
	if err != nil {
		return nil, errors.Join(errors.New("Chromaprint extraction failed"),
			s.songs.MarkFingerprintAttempted(ctx, song.ID, attemptedAt, "Chromaprint extraction failed"))
	}
	if duration <= 0 {
		duration = song.Duration
	}
	if err := s.songs.UpdateFingerprint(ctx, song.ID, fp, duration, attemptedAt); err != nil {
		return nil, err
	}
	song.Fingerprint, song.FingerprintDuration = fp, duration
	return song, nil
}
