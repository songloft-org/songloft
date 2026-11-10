package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"songloft/internal/models"
	"testing"
	"time"
)

func TestBridgePlaylistMemberTimeFullAndBrief(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermPlaylistsRead}, "member-time")
	ctx := context.Background()
	song := &models.Song{Title: "time", Type: models.TypeRemote, URL: "https://example.com/time"}
	if err := h.db.SongRepository().Create(ctx, song); err != nil {
		t.Fatal(err)
	}
	if err := h.db.PlaylistSongRepository().AddSong(ctx, 1, song.ID, 1); err != nil {
		t.Fatal(err)
	}
	for _, brief := range []bool{false, true} {
		payload, err := h.HandleBridgeCall("playlists.getSongs", fmt.Sprintf(`{"id":1,"options":{"brief":%t}}`, brief))
		if err != nil {
			t.Fatal(err)
		}
		var rows []map[string]any
		if err := json.Unmarshal([]byte(payload), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("wrong songs %s", payload)
		}
		value, ok := rows[0]["playlist_added_at"].(string)
		if !ok {
			t.Fatalf("no member time: %s", payload)
		}
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			t.Fatal(err)
		}
		if !brief && rows[0]["added_at"] == nil {
			t.Fatalf("library time removed: %s", payload)
		}
	}
}
