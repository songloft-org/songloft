package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/hanxi/tag"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestSongTagAndLyricEditsPreserveFileTags(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	repo := db.SongRepository()
	service := services.NewSongService(repo, db, nil, nil, nil, nil)
	tagService := services.NewSongTagService(db.SongTagRepository(), db)
	service.SetSongTagService(tagService)
	h := NewSongHandler(service, nil, nil, nil, nil, nil)
	router := chi.NewRouter()
	router.Put("/songs/{id}/tags", h.WriteTags)
	router.Put("/songs/{id}/lyrics", h.UpdateSongLyrics)
	fixture, err := os.ReadFile("../../pkg/tag/testdata/without_tags/sample.ogg")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "song.ogg")
	if err := os.WriteFile(path, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	err = tag.WriteTag(path, tag.WriteOptions{
		Title: "File title", Artist: "File artist", AlbumArtist: "Various Artists", Track: "6/20", Disc: "2/3",
		CustomTags: map[string]string{"KEEP_ME": "custom value", "SONGLOFT_TAGS": "old label"},
	})
	if err != nil {
		t.Fatal(err)
	}
	song := &models.Song{Type: models.TypeLocal, FilePath: path, Title: "DB title", Artist: "DB artist", Year: 2026}
	seedSong(t, repo, song)
	put := func(route, body, statusKey, want string) {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/songs/%d/%s", song.ID, route), strings.NewReader(body)))
		var response map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != http.StatusOK || string(response[statusKey]) != fmt.Sprintf("%q", want) {
			t.Fatalf("%s: %d %s", route, w.Code, w.Body.String())
		}
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	put("tags", `{"title":""}`, "file_write", "unchanged")
	afterNoop, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, afterNoop) {
		t.Fatalf("empty edit changed the file: %v", err)
	}
	put("tags", `{"title":"Edited title"}`, "file_write", "written")
	put("lyrics", `{"lyric_source":"manual","lyric":"[00:01]Line"}`, "file_write_status", "written")
	ctx := context.Background()
	if err := tagService.BindByNames(ctx, song.ID, []string{"new label"}); err != nil {
		t.Fatal(err)
	}
	service.SyncSongTagsToFile(ctx, song.ID)
	checkFile := func(label string) {
		t.Helper()
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		m, err := tag.ReadFrom(f)
		if err != nil {
			t.Fatal(err)
		}
		n, total := m.Track()
		d, discs := m.Disc()
		labelValue, _ := m.Raw()["songloft_tags"].(string)
		if m.Title() != "Edited title" || m.Artist() != "File artist" || m.AlbumArtist() != "Various Artists" ||
			m.Year() != 0 || m.Lyrics() != "[00:01]Line" || n != 6 || total != 20 || d != 2 || discs != 3 ||
			m.Raw()["keep_me"] != "custom value" || labelValue != label {
			t.Fatalf("unexpected file tags: %+v", m.Raw())
		}
	}
	checkFile("new label")
	if err := tagService.SetSongTags(ctx, song.ID, nil); err != nil {
		t.Fatal(err)
	}
	service.SyncSongTagsToFile(ctx, song.ID)
	checkFile("")
	stored, err := repo.GetByID(ctx, song.ID)
	const wantOverrides = models.MetadataOverrideTitle | models.MetadataOverrideLyrics
	if err != nil || stored.Title != "Edited title" || stored.MetadataOverrideMask&wantOverrides != wantOverrides {
		t.Fatalf("missing persistent edits: %+v %v", stored, err)
	}
	put("tags", "{ }", "file_write", "written")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		t.Fatal(err)
	}
	if m.Artist() != "DB artist" || m.Year() != 2026 || m.AlbumArtist() != "Various Artists" ||
		m.Raw()["keep_me"] != "custom value" {
		t.Fatalf("unexpected synced tags: %+v", m.Raw())
	}
}

func TestAlbumOrder(t *testing.T) {
	repo := newTestSongRepo(t)
	h := NewSongHandler(services.NewSongService(repo, nil, nil, nil, nil, nil), nil, nil, nil, nil, nil)
	ctx := context.Background()
	// Deliberately insert out of order. Filenames and titles also disagree
	// with track order, and disc 2 restarts its numbering.
	rows := []*models.Song{
		{Title: "disc2", Track: "1/12", Disc: "02/02"},
		{Title: "ten", Track: "10/12"},
		{Title: "unknown", Track: "invalid"},
		{Title: "two", Track: " 02 /12", Disc: "1/2"},
		{Title: "cue", CueTrackIndex: 3},
		{Title: "one", Track: "1"},
		{Title: "zero", Track: "0"},
		{Title: "other", Track: "1", Album: "Other"},
	}
	for i, song := range rows {
		song.Type = models.TypeLocal
		song.FilePath = fmt.Sprintf("/music/%02d.mp3", i)
		if song.Album == "" {
			song.Album = "Album"
		}
		seedSong(t, repo, song)
	}
	want := []int64{rows[5].ID, rows[3].ID, rows[4].ID, rows[1].ID, rows[2].ID, rows[6].ID, rows[0].ID}
	list := func(query string) []int64 {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ListSongs(rr, httptest.NewRequest(http.MethodGet, "/songs?"+query, nil))
		var response struct{ Songs []*models.Song }
		if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &response) != nil {
			t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
		}
		ids := make([]int64, len(response.Songs))
		for i, song := range response.Songs {
			ids[i] = song.ID
		}
		return ids
	}
	for _, query := range []string{"album=Album", "album=Album&sort=track", "album=Album&sort=track&order=asc"} {
		if got := list(query); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v want %v", query, got, want)
		}
	}
	var paged []int64
	for offset := 0; offset < len(want); offset += 2 {
		paged = append(paged, list(fmt.Sprintf("album=Album&limit=2&offset=%d", offset))...)
	}
	if !reflect.DeepEqual(paged, want) {
		t.Errorf("paged: got %v want %v", paged, want)
	}
	rr := httptest.NewRecorder()
	h.ListSongIDs(rr, httptest.NewRequest(http.MethodGet, "/songs/ids?album=Album", nil))
	var response struct{ IDs []int64 }
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.IDs, want) {
		t.Errorf("IDs: got %v want %v", response.IDs, want)
	}
	// Explicit sorting still takes precedence over the album default.
	got := list("album=Album&sort=id&order=desc")
	explicit := []int64{rows[6].ID, rows[5].ID, rows[4].ID, rows[3].ID, rows[2].ID, rows[1].ID, rows[0].ID}
	if !reflect.DeepEqual(got, explicit) {
		t.Errorf("explicit: %v", got)
	}
	desc := []int64{rows[0].ID, rows[1].ID, rows[4].ID, rows[3].ID, rows[5].ID, rows[2].ID, rows[6].ID}
	if got := list("album=Album&sort=track&order=desc"); !reflect.DeepEqual(got, desc) {
		t.Errorf("desc: %v", got)
	}
	// An updated tag must affect the next query, including ID-only queues.
	rows[1].Track = "4"
	rows[1].Disc = "2"
	if err := repo.Update(ctx, rows[1]); err != nil {
		t.Fatal(err)
	}
	want = []int64{rows[5].ID, rows[3].ID, rows[4].ID, rows[2].ID, rows[6].ID, rows[0].ID, rows[1].ID}
	if got := list("album=Album"); !reflect.DeepEqual(got, want) {
		t.Errorf("updated: %v", got)
	}
	// Repository callers have the same default without the HTTP parser.
	songs, err := repo.List(ctx, &database.SongFilter{Album: "Album"})
	if err != nil {
		t.Fatal(err)
	}
	if songs[len(songs)-1].Disc != "2" {
		t.Errorf("disc round trip: %+v", songs[len(songs)-1])
	}
	ids, err := repo.ListIDs(ctx, &database.SongFilter{Album: "Album"})
	if err != nil || !reflect.DeepEqual(ids, want) {
		t.Errorf("repository IDs: %v %v", ids, err)
	}
}

func TestAlbumOrderAfterWritingTags(t *testing.T) {
	repo := newTestSongRepo(t)
	h := NewSongHandler(services.NewSongService(repo, nil, nil, nil, nil, nil), nil, nil, nil, nil, nil)
	fixture, err := os.ReadFile("../../pkg/tag/testdata/without_tags/sample.mp3")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(path, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	song := &models.Song{Type: models.TypeLocal, FilePath: path, Title: "Song", Album: "Album", Track: "10", Disc: "2/2"}
	seedSong(t, repo, song)
	other := &models.Song{Type: models.TypeLocal, FilePath: "/other.mp3", Title: "Other", Album: "Album", Track: "2"}
	seedSong(t, repo, other)
	router := chi.NewRouter()
	router.Put("/songs/{id}/tags", h.WriteTags)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/songs/%d/tags", song.ID), strings.NewReader(`{"track":"01/12","disc":"1/2"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("write: %d %s", rr.Code, rr.Body.String())
	}
	var response struct {
		FileWrite string `json:"file_write"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.FileWrite != "written" {
		t.Fatalf("file write: %s", response.FileWrite)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	meta, err := tag.ReadFrom(f)
	if err != nil {
		t.Fatal(err)
	}
	if num, total := meta.Track(); num != 1 || total != 12 {
		t.Errorf("track: %d/%d", num, total)
	}
	if num, total := meta.Disc(); num != 1 || total != 2 {
		t.Errorf("disc: %d/%d", num, total)
	}
	ids, err := repo.ListIDs(context.Background(), &database.SongFilter{Album: "Album"})
	if err != nil || !reflect.DeepEqual(ids, []int64{song.ID, other.ID}) {
		t.Errorf("after edit: %v %v", ids, err)
	}
}
