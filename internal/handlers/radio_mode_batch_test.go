package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services"
)

func TestBatchUpdateRadioMode(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	radioA := &models.Song{Type: models.TypeRadio, Title: "A", URL: "https://example.com/a", IsVideo: true}
	radioB := &models.Song{Type: models.TypeRadio, Title: "B", URL: "https://example.com/b", IsVideo: false}
	local := &models.Song{Type: models.TypeLocal, Title: "Local", FilePath: "/music/local.mp3"}
	for _, song := range []*models.Song{radioA, radioB, local} {
		if err := db.SongRepository().Create(context.Background(), song); err != nil {
			t.Fatalf("create %s: %v", song.Title, err)
		}
	}

	handler := NewSongHandler(services.NewSongService(db.SongRepository(), nil, nil, nil, nil, nil), nil, nil, nil, nil, nil)
	router := chi.NewRouter()
	router.Post("/songs/batch-radio-mode", handler.BatchUpdateRadioMode)

	request := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/songs/batch-radio-mode", strings.NewReader(body)))
		return recorder
	}

	response := request(`{"ids":[1,2,99999],"radio_mode":"hls","is_video":true}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	var result models.BatchUpdateRadioModeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Updated != 2 {
		t.Fatalf("updated = %d, want 2", result.Updated)
	}
	for _, id := range []int64{radioA.ID, radioB.ID} {
		song, err := db.SongRepository().GetByID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if song.RadioMode != models.RadioModeHLS || !song.IsVideo {
			t.Fatalf("song %d = %+v", id, song)
		}
	}

	response = request(`{"ids":[1,3],"radio_mode":"audio_only"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("mixed type status = %d, body = %s", response.Code, response.Body)
	}
	response = request(`{"ids":[99999],"radio_mode":"invalid"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode status = %d, body = %s", response.Code, response.Body)
	}
	response = request(`{"ids":[1,2],"radio_mode":"audio_only"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("audio-only status = %d, body = %s", response.Code, response.Body)
	}
	for _, id := range []int64{radioA.ID, radioB.ID} {
		song, err := db.SongRepository().GetByID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if song.RadioMode != models.RadioModeAudioOnly || song.IsVideo {
			t.Fatalf("song %d after audio-only = %+v", id, song)
		}
	}
}
