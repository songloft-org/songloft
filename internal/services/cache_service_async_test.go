package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"songloft/internal/models"
)

func TestAsyncDownloadAndCacheSharesFullDownloads(t *testing.T) {
	for _, playbackFirst := range []bool{false, true} {
		name := "prefetch first"
		if playbackFirst {
			name = "supplement first"
		}
		t.Run(name, func(t *testing.T) {
			cache, repo := newValidationCache(t)
			data := validationWAV(40, 8000)
			started := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseOrigin := func() { releaseOnce.Do(func() { close(release) }) }
			var requests atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					close(started)
					<-release
				}
				w.Header().Set("Content-Type", "audio/wav")
				_, _ = w.Write(data)
			}))
			defer origin.Close()
			defer releaseOrigin()
			cache.downloadClient = origin.Client()
			song := &models.Song{Type: models.TypeRemote, URL: origin.URL, Duration: 40}
			if err := repo.Create(t.Context(), song); err != nil {
				t.Fatal(err)
			}

			finished := make(chan error, 1)
			go func() {
				if playbackFirst {
					cache.AsyncDownloadAndCache(t.Context(), song, origin.URL, nil)
					finished <- nil
					return
				}
				_, err := cache.Get(t.Context(), song)
				finished <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("the first download did not reach the origin")
			}

			if playbackFirst {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				defer cancel()
				if _, err := cache.Get(ctx, song); !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("Get should wait for the supplemental download and honor cancellation, got %v", err)
				}
			}

			supplementDone := make(chan struct{})
			go func() {
				cache.AsyncDownloadAndCache(t.Context(), song, origin.URL, nil)
				close(supplementDone)
			}()
			select {
			case <-supplementDone:
			case <-time.After(5 * time.Second):
				t.Error("redundant supplemental download should return without waiting")
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("requests while first download is in progress: %d, want 1", got)
			}

			releaseOrigin()
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the first download did not finish")
			}
			if _, ok := cache.FindCachedFileBySong(song); !ok {
				t.Fatal("expected a valid full-song cache entry")
			}
			cache.AsyncDownloadAndCache(t.Context(), song, origin.URL, nil)
			if got := requests.Load(); got != 1 {
				t.Errorf("a late 206 should reuse the completed cache, got %d origin requests", got)
			}
		})
	}
}
