package jsplugin

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"songloft/internal/services"
)

func (h *BridgeHandler) handleLocalSongs(action, data string) (string, error) {
	if h.songService == nil {
		return "", errors.New("local song service is unavailable")
	}
	var args struct {
		services.LocalMetadataUpdate
		OperationID   string  `json:"operation_id"`
		Duration      float64 `json:"duration"`
		Fingerprint   string  `json:"fingerprint"`
		WantLyrics    bool    `json:"lookup_lyrics"`
		WantCover     bool    `json:"lookup_cover"`
		IncludeTracks *bool   `json:"include_tracks"`
	}
	if err := json.Unmarshal([]byte(data), &args); err != nil {
		return "", err
	}
	if action == "songs.cancelLocalOperation" {
		h.localMu.Lock()
		if cancel := h.localOperations[args.OperationID]; cancel != nil {
			cancel()
		}
		h.localMu.Unlock()
		return "{}", nil
	}

	ctx, done, err := h.beginLocalOperation(args.OperationID)
	if err != nil {
		return "", err
	}
	defer done()

	var result any
	switch action {
	case "songs.localLibrary":
		root, err := h.songService.LocalLibraryRoot()
		if err != nil {
			return "", err
		}
		if args.IncludeTracks != nil && !*args.IncludeTracks {
			result = map[string]any{"root": root, "fingerprint_available": services.IsChromaprintAvailable()}
			break
		}
		rows, err := h.db.SongRepository().ListIndexedLocalTracks(ctx)
		if err != nil {
			return "", err
		}
		tracks := make([]services.IndexedLocalTrack, 0, len(rows))
		for _, row := range rows {
			rel, err := filepath.Rel(root, row.FilePath)
			if err == nil && rel != "." && filepath.IsLocal(rel) {
				tracks = append(tracks, services.IndexedLocalTrack{ID: row.ID, FilePath: row.FilePath,
					RelativePath: filepath.ToSlash(rel), Title: row.Title, Artist: row.Artist, Album: row.Album})
			}
		}
		result = map[string]any{"root": root, "tracks": tracks, "fingerprint_available": services.IsChromaprintAvailable()}
	case "songs.inspectLocalFile":
		file, failure := h.songService.InspectLocalFile(ctx, args.SongID)
		if failure != nil {
			result = map[string]string{"state": failure.State, "detail": failure.Error()}
		} else {
			result = file
		}
	case "songs.ensureFingerprint":
		if !CheckPermission(h.permissions, PermSongsRead) {
			return "", errors.New("permission denied: requires 'songs.read'")
		}
		song, err := h.songService.EnsureLocalFingerprint(ctx, args.SongID)
		if err != nil {
			return "", err
		}
		result = map[string]any{"fingerprint": song.Fingerprint, "fingerprint_duration": song.FingerprintDuration}
	case "songs.lookupMetadata":
		media := make(map[string]any)
		artist := strings.Join(args.Artists, " & ")
		if args.WantLyrics && h.lookupLyrics != nil {
			if lyrics, err := h.lookupLyrics(ctx, args.Title, artist, args.Album, args.Duration, args.Fingerprint, args.ISRC); err == nil {
				media["lyrics"] = lyrics
			}
		}
		if args.WantCover && h.lookupCover != nil {
			if cover, err := h.lookupCover(ctx, args.Title, artist, args.Album, args.Fingerprint, args.ISRC); err == nil {
				media["cover_url"] = cover
			}
		}
		result = media
	case "songs.writeLocalMetadata":
		if !CheckPermission(h.permissions, PermSongsRead) {
			return "", errors.New("permission denied: requires 'songs.read'")
		}
		result = h.songService.WriteLocalMetadata(ctx, args.LocalMetadataUpdate)
	}
	out, err := json.Marshal(result)
	return string(out), err
}

func (h *BridgeHandler) beginLocalOperation(id string) (context.Context, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	if id == "" {
		return ctx, cancel, nil
	}
	h.localMu.Lock()
	defer h.localMu.Unlock()
	if h.localClosed || len(id) > 128 || h.localOperations[id] != nil {
		cancel()
		return nil, nil, errors.New("invalid or unavailable local operation")
	}
	if h.localOperations == nil {
		h.localOperations = make(map[string]context.CancelFunc)
	}
	h.localOperations[id] = cancel
	return ctx, func() {
		cancel()
		h.localMu.Lock()
		delete(h.localOperations, id)
		h.localMu.Unlock()
	}, nil
}

func (h *BridgeHandler) cancelLocalOperations() {
	h.localMu.Lock()
	defer h.localMu.Unlock()
	h.localClosed = true
	for _, cancel := range h.localOperations {
		cancel()
	}
}
