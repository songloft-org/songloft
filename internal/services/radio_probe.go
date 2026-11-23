package services

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"songloft/internal/models"
)

const (
	radioProbeBudget  = 3 * time.Second
	radioProbeWorkers = 4
)

// RadioSourceProbe checks a small prefix from otherwise unclassified live sources.
type RadioSourceProbe struct {
	allowlist func() []string
	slots     chan struct{}
}

func NewRadioSourceProbe(allowlist func() []string) *RadioSourceProbe {
	return &RadioSourceProbe{allowlist: allowlist, slots: make(chan struct{}, radioProbeWorkers)}
}

func (p *RadioSourceProbe) detect(ctx context.Context, source string) models.RadioMode {
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	case <-ctx.Done():
		return models.RadioModeAuto
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return models.RadioModeAuto
	}
	gateway := &hlsPullGateway{allowlist: p.allowlist}
	if gateway.check(ctx, req.URL) != nil || gateway.initSourceClient() != nil {
		return models.RadioModeAuto
	}
	defer gateway.transport.CloseIdleConnections()
	req.Header.Set("User-Agent", "Songloft/1.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", req.URL.Scheme+"://"+req.URL.Host+"/")
	resp, err := gateway.client.Do(req)
	if err != nil {
		return models.RadioModeAuto
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return models.RadioModeAuto
	}
	var prefix [64]byte
	for used := 0; used < len(prefix); {
		n, readErr := resp.Body.Read(prefix[used:])
		used += n
		content := bytes.TrimSpace(bytes.TrimPrefix(prefix[:used], []byte{0xef, 0xbb, 0xbf}))
		switch {
		case bytes.HasPrefix(content, []byte("#EXTM3U")):
			return models.RadioModeHLS
		case bytes.HasPrefix(content, []byte("FLV")):
			return models.RadioModeFLV
		}
		if readErr != nil {
			break
		}
	}
	return models.RadioModeAuto
}

// RefreshRadioModes runs within the write request's shared time budget. Positive
// results are conditional on the station still being in auto mode with this URL.
func (s *SongService) RefreshRadioModes(ctx context.Context, songs []*models.Song) {
	if s.radioProbe == nil {
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, radioProbeBudget)
	defer cancel()
	var probes errgroup.Group
	probes.SetLimit(radioProbeWorkers)
	for _, song := range songs {
		if probeCtx.Err() != nil {
			break
		}
		if song.Type != models.TypeRadio || song.RadioMode != models.RadioModeAuto || song.URL == "" || models.RadioModeFromURL(song.URL) != models.RadioModeAuto {
			continue
		}
		probes.Go(func() error {
			mode := s.radioProbe.detect(probeCtx, song.URL)
			if mode == models.RadioModeAuto {
				return nil
			}
			updated, err := s.songs.UpdateRadioDetectedMode(probeCtx, song.ID, song.URL, mode)
			if err != nil && probeCtx.Err() == nil {
				slog.Debug("radio mode detection could not be saved", "song_id", song.ID)
			}
			if updated {
				song.RadioDetectedMode = mode
			}
			return nil
		})
	}
	_ = probes.Wait()
}
