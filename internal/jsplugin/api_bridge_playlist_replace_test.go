package jsplugin

import (
	"context"
	"fmt"
	"songloft/internal/models"
	"songloft/internal/services"
	"testing"
)

func TestBridgeReplaceSongsRequiresArrayAndWritePermission(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermPlaylistsWrite}, "navidrome")
	h.playlistService = services.NewPlaylistService(h.db.PlaylistRepository(), h.db.PlaylistSongRepository(), h.db.SongRepository(), nil)
	h.playlistService.SetTransactor(h.db)
	ctx := context.Background()
	playlist := &models.Playlist{Name: "sync", Type: models.PlaylistTypeNormal}
	if err := h.db.PlaylistRepository().Create(ctx, playlist); err != nil {
		t.Fatal(err)
	}
	song := &models.Song{Title: "keep", Type: models.TypeRemote, URL: "https://example.com/audio"}
	if err := h.db.SongRepository().Create(ctx, song); err != nil {
		t.Fatal(err)
	}
	if err := h.db.PlaylistSongRepository().AddSong(ctx, playlist.ID, song.ID, 1); err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{fmt.Sprintf(`{"id":%d}`, playlist.ID), fmt.Sprintf(`{"id":%d,"songIds":null}`, playlist.ID)} {
		if _, err := h.HandleBridgeCall("playlists.replaceSongs", payload); err == nil {
			t.Fatal("missing array accepted")
		}
	}
	got, err := h.db.PlaylistSongRepository().GetSongs(ctx, playlist.ID)
	if err != nil || len(got) != 1 {
		t.Fatalf("membership changed: %v %v", got, err)
	}
	h.permissions = []string{PermPlaylistsRead}
	payload := fmt.Sprintf(`{"id":%d,"songIds":[]}`, playlist.ID)
	if _, err := h.HandleBridgeCall("playlists.replaceSongs", payload); err == nil {
		t.Fatal("read-only plugin replaced membership")
	}
	h.permissions = []string{PermPlaylistsWrite}
	if _, err := h.HandleBridgeCall("playlists.replaceSongs", payload); err != nil {
		t.Fatal(err)
	}
	got, err = h.db.PlaylistSongRepository().GetSongs(ctx, playlist.ID)
	if err != nil || len(got) != 0 {
		t.Fatalf("explicit empty array did not clear: %v %v", got, err)
	}
}
