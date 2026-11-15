package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
	"songloft/internal/services/source"
)

// Real PCM, including its complete payload; no mocked probe or database.
func validationWAV(duration, sampleRate int) []byte {
	const channels, bits = 1, 8
	size := duration * sampleRate
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+size))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(channels))
	binary.Write(&b, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&b, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(bits))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(size))
	b.Write(bytes.Repeat([]byte{128}, size))
	return b.Bytes()
}

func newValidationCache(t *testing.T) (*CacheService, *database.SongRepository) {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	c := NewCacheService(t.TempDir(), NewConfigService(db.ConfigRepository()))
	t.Cleanup(c.Shutdown)
	repo := db.SongRepository()
	c.SetCachePathCallbacks(repo.UpdateCachePath, repo.ClearCachePath, repo.ClearAllCachePaths, repo.ListSongsWithCache)
	return c, repo
}

func validationTempFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audio.wav")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCacheValidationFinalize(t *testing.T) {
	for _, tc := range []struct {
		name     string
		data     []byte
		expected float64
	}{
		{"prompt", validationWAV(10, 8000), 299},
		{"duration low", validationWAV(40, 8000), 299},
		{"duration high", validationWAV(100, 8000), 40},
		{"bitrate low", validationWAV(40, 100), 40},
		{"unreadable", bytes.Repeat([]byte("not audio"), 200), 299},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, repo := newValidationCache(t)
			song := &models.Song{Type: models.TypeRemote, URL: "https://example.invalid/audio", Duration: tc.expected}
			if err := repo.Create(context.Background(), song); err != nil {
				t.Fatal(err)
			}
			path := validationTempFile(t, tc.data)
			c.FinalizeCache(context.Background(), song, path, ".wav")
			stored, err := repo.GetByID(context.Background(), song.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.CachePath != "" {
				t.Errorf("invalid audio persisted: %s", stored.CachePath)
			}
			if _, ok := c.findCachedFileBySong(song); ok {
				t.Error("invalid audio moved into music_cache")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("invalid temporary file retained: %v", err)
			}
		})
	}
}

func TestCacheValidationExternalGet(t *testing.T) {
	for _, duration := range []int{10, 40} {
		t.Run((time.Duration(duration) * time.Second).String(), func(t *testing.T) {
			data := validationWAV(duration, 8000)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "audio/wav")
				w.Write(data)
			}))
			defer srv.Close()
			c, _ := newValidationCache(t)
			song := &models.Song{ID: 570, Type: models.TypeRemote, URL: srv.URL, Duration: 40}
			path, err := c.Get(context.Background(), song)
			if duration == 10 {
				var invalid *source.InvalidAudioError
				if !errors.As(err, &invalid) || invalid.Reason != source.ReasonTooShort {
					t.Fatalf("expected too_short, got path=%q err=%v", path, err)
				}
				if _, ok := c.findCachedFileBySong(song); ok {
					t.Error("invalid external audio cached")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				actual, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(actual, data) {
					t.Fatalf("valid external audio changed: %v", err)
				}
			}
		})
	}
}

func TestCacheValidationExistingHit(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "structured", true: "legacy"}[legacy], func(t *testing.T) {
			c, repo := newValidationCache(t)
			good := validationWAV(40, 8000)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "audio/wav")
				w.Write(good)
			}))
			defer srv.Close()
			song := &models.Song{Type: models.TypeRemote, URL: srv.URL, Duration: 40}
			if err := repo.Create(context.Background(), song); err != nil {
				t.Fatal(err)
			}
			dir, base := c.getCachePath(song.ID, cacheKeyOf(song))
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			badPath := filepath.Join(dir, base+".wav")
			if err := os.WriteFile(badPath, validationWAV(10, 8000), 0600); err != nil {
				t.Fatal(err)
			}
			derivative := filepath.Join(dir, base+".tc.mp3")
			if err := os.WriteFile(derivative, []byte("bad derivative"), 0600); err != nil {
				t.Fatal(err)
			}
			if !legacy {
				song.CachePath = badPath
				if err := repo.UpdateCachePath(context.Background(), song.ID, badPath); err != nil {
					t.Fatal(err)
				}
			}
			path, err := c.Get(context.Background(), song)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, good) {
				t.Fatalf("bad cache returned instead of refetch: %v (%d bytes)", err, len(actual))
			}
			stored, err := repo.GetByID(context.Background(), song.ID)
			if err != nil || stored.CachePath != "" {
				t.Fatalf("stale cache_path not cleared: %+v %v", stored, err)
			}
			if _, err := os.Stat(derivative); !os.IsNotExist(err) {
				t.Errorf("bad derivative retained: %v", err)
			}
		})
	}
}

func TestCacheValidationRecheck(t *testing.T) {
	c, _ := newValidationCache(t)
	path := validationTempFile(t, validationWAV(40, 8000))
	song := &models.Song{ID: 570, Type: models.TypeRemote, URL: "https://example.invalid/song", Duration: 40, CachePath: path}
	if _, ok := c.FindCachedFileBySong(song); !ok {
		t.Fatal("valid cache rejected")
	}
	// Expected duration and runtime policy changes must apply to memoized files.
	song.Duration = 299
	if _, ok := c.FindCachedFileBySong(song); ok {
		t.Fatal("changed expected duration bypassed validation")
	}
	if err := os.WriteFile(path, validationWAV(40, 8000), 0600); err != nil {
		t.Fatal(err)
	}
	song.Duration = 40
	if _, ok := c.FindCachedFileBySong(song); !ok {
		t.Fatal("restored valid cache rejected")
	}
	opts := source.DefaultValidationOpts()
	opts.MinDuration = 50
	if err := c.configService.SetJSON("source_validation", opts); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.FindCachedFileBySong(song); ok {
		t.Fatal("changed validation policy bypassed")
	}
	opts.Enabled = false
	if err := c.configService.SetJSON("source_validation", opts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, validationWAV(10, 8000), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.FindCachedFileBySong(song); !ok {
		t.Fatal("disabled validation still rejects audio")
	}
	opts = source.DefaultValidationOpts()
	if err := c.configService.SetJSON("source_validation", opts); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.FindCachedFileBySong(song); ok {
		t.Fatal("reenabled validation bypassed")
	}
}

func TestCacheValidationFileReplacement(t *testing.T) {
	c, _ := newValidationCache(t)
	path := validationTempFile(t, validationWAV(40, 8000))
	song := &models.Song{ID: 570, Type: models.TypeRemote, URL: "https://example.invalid/song", Duration: 40, CachePath: path}
	if _, ok := c.FindCachedFileBySong(song); !ok {
		t.Fatal("valid cache rejected")
	}
	// Equal size and mtime, different inode and duration: a replaced file must be probed.
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := validationWAV(10, 8000)
	bad = append(bad, make([]byte, int(before.Size())-len(bad))...)
	replacement := validationTempFile(t, bad)
	if err := os.Chtimes(replacement, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.FindCachedFileBySong(song); ok {
		t.Fatal("replaced prompt reused old validation")
	}
}

func TestCacheValidationCanceledHitDoesNotEvict(t *testing.T) {
	c, _ := newValidationCache(t)
	path := validationTempFile(t, validationWAV(10, 8000))
	song := &models.Song{ID: 570, Type: models.TypeRemote, CachePath: path}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.findValidatedCache(ctx, song)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("canceled request evicted cache: %v", err)
	}
}

type validationProber struct{ extractor *MetadataExtractor }

func (p validationProber) ProbeForValidation(ctx context.Context, path string) (source.AudioInfoLike, error) {
	return p.extractor.ProbeForValidation(ctx, path)
}

type validationPlugins struct{ url string }

func (p validationPlugins) ListActiveEntryPaths() []string { return []string{"primary", "alternate"} }
func (p validationPlugins) InvokeHTTP(ctx context.Context, entry, method, path string, query interface{}, body []byte) (int, map[string]string, []byte, error) {
	if path == "/api/search" {
		return http.StatusOK, nil, []byte(`{"results":[{"title":"test song","artist":"test artist","duration":40,"source_data":{"id":"good"}}]}`), nil
	}
	url := p.url + "/bad"
	if entry == "alternate" {
		url = p.url + "/good"
	}
	data, _ := json.Marshal(map[string]string{"url": url})
	return http.StatusOK, nil, data, nil
}

func TestCacheValidationTriggersSourceFallback(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "finalize", true: "old cache"}[existing], func(t *testing.T) {
			c, repo := newValidationCache(t)
			c.SetCacheCompleteCallback(nil)
			tmpDir := t.TempDir()
			t.Setenv("TMPDIR", tmpDir)
			bad, good := validationWAV(10, 8000), validationWAV(40, 8000)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "audio/wav")
				if r.URL.Path == "/good" {
					w.Write(good)
				} else {
					w.Write(bad)
				}
			}))
			defer srv.Close()
			plugins := validationPlugins{url: srv.URL}
			metrics := source.NewSourceMetrics(source.DefaultMetricsOpts())
			extractor := NewMetadataExtractor(&MetadataConfig{})
			fetcher := source.NewSourceFetcher(source.FetcherOpts{Prober: validationProber{extractor}, PluginInvoker: plugins, Metrics: metrics, HTTPClient: srv.Client()})
			resolver := source.NewSourceResolver(plugins, plugins, metrics, source.DefaultResolverOpts())
			s := NewSongService(repo, nil, nil, nil, nil, nil)
			orch := source.NewSourceOrchestrator(source.OrchestratorOpts{
				Fetcher: fetcher, Resolver: resolver, SongUpdater: s,
				SameSourceRetries: 1, SameSourceRetryBackoff: time.Millisecond,
				FallbackInterval: time.Nanosecond, FallbackJitter: time.Nanosecond,
			})
			c.SetOrchestrator(orch)
			var rejected atomic.Int32
			c.SetSourceValidation(extractor, metrics, func(song *models.Song) {
				rejected.Add(1)
				orch.AsyncReassign(song.ID, source.ReassignSessionKey{}, func(ctx context.Context, id int64) (*source.SongInfo, error) {
					song, err := repo.GetByID(ctx, id)
					return songInfoOf(song), err
				})
			})
			song := &models.Song{Type: models.TypeRemote, Title: "test song", Artist: "test artist", Duration: 40, PluginEntryPath: "primary", SourceData: `{"id":"bad"}`, DedupKey: "test"}
			if err := repo.Create(context.Background(), song); err != nil {
				t.Fatal(err)
			}
			path := validationTempFile(t, bad)
			if existing {
				song.CachePath = path
				if err := repo.UpdateCachePath(context.Background(), song.ID, path); err != nil {
					t.Fatal(err)
				}
				if _, ok := c.FindCachedFileBySong(song); ok {
					t.Fatal("old prompt accepted")
				}
			} else {
				c.FinalizeCache(context.Background(), song, path, ".wav")
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				stored, err := repo.GetByID(context.Background(), song.ID)
				if err != nil {
					t.Fatal(err)
				}
				if stored.PluginEntryPath == "alternate" {
					song = stored
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("source fallback did not persist replacement: %+v", stored)
				}
				time.Sleep(10 * time.Millisecond)
			}
			if rejected.Load() == 0 {
				t.Fatal("no invalid audio feedback")
			}
			foundFailure := false
			for _, health := range metrics.Snapshot(10) {
				for _, failure := range health.LastFailures {
					if failure.Result == source.OutcomeValidationFail && failure.Reason == string(source.ReasonTooShort) {
						foundFailure = true
					}
				}
			}
			if !foundFailure {
				t.Fatal("validation failure absent from source health metrics")
			}
			path, err := c.Get(context.Background(), song)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, good) {
				t.Fatalf("fallback audio invalid: %v", err)
			}
			for {
				files, err := os.ReadDir(tmpDir)
				if err != nil {
					t.Fatal(err)
				}
				if len(files) == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("fallback temporary files leaked: %v", files)
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}

func TestCacheValidationConcurrentHits(t *testing.T) {
	c, _ := newValidationCache(t)
	path := validationTempFile(t, validationWAV(40, 8000))
	song := &models.Song{ID: 570, Type: models.TypeRemote, URL: "https://example.invalid/song", Duration: 40, CachePath: path}
	results := make(chan bool, 16)
	for i := 0; i < cap(results); i++ {
		go func() { _, ok := c.FindCachedFileBySong(song); results <- ok }()
	}
	for i := 0; i < cap(results); i++ {
		if !<-results {
			t.Fatal("concurrent valid cache hit rejected")
		}
	}
	// An in-place rewrite must invalidate the remembered technical parameters.
	if err := os.WriteFile(path, validationWAV(10, 8000), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.FindCachedFileBySong(song); ok {
		t.Fatal("modified prompt reused old validation")
	}
}
