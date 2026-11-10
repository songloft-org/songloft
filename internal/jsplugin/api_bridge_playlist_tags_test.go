package jsplugin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"songloft/internal/services"
)

func TestBridgeSharedPlaylistTagsPermissionsAndSystemLabels(t *testing.T) {
	h := newTestBridgeHandler(t, []string{PermTagsWrite, PermTagsRead}, "navidrome")
	h.songTagService = services.NewSongTagService(h.db.SongTagRepository(), h.db)
	ctx := context.Background()
	tag, err := h.songTagService.Create(ctx, "Navidrome", "")
	if err != nil {
		t.Fatal(err)
	}
	payload := fmt.Sprintf(`{"tagId":%d,"playlistIds":[1]}`, tag.ID)
	result, err := h.HandleBridgeCall("tags.bindPlaylists", payload)
	if err != nil || !strings.Contains(result, `"bound":1`) {
		t.Fatalf("bind: %s %v", result, err)
	}
	h.permissions = []string{PermTagsRead}
	if _, err := h.HandleBridgeCall("tags.unbindPlaylists", payload); err == nil {
		t.Fatal("read-only plugin unbound tags")
	}
	result, err = h.HandleBridgeCall("tags.getPlaylistTags", `{"playlistId":1}`)
	if err != nil || !strings.Contains(result, "Navidrome") {
		t.Fatalf("read: %s %v", result, err)
	}
	h.permissions = []string{PermTagsWrite}
	if _, err := h.HandleBridgeCall("tags.getPlaylistTags", `{"playlistId":1}`); err == nil {
		t.Fatal("write-only plugin read tags")
	}
	if _, err := h.HandleBridgeCall("tags.unbindPlaylists", payload); err != nil {
		t.Fatal(err)
	}
	p, err := h.db.PlaylistRepository().GetByID(ctx, 1)
	if err != nil || len(p.Labels) != 1 || p.Labels[0] != "built_in" {
		t.Fatalf("system label changed: %+v %v", p, err)
	}
}
