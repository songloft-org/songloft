package services

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hanxi/tag"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

type localMetadataTransport func(*http.Request) (*http.Response, error)

func (f localMetadataTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newLocalMetadataFixture(t *testing.T) (*SongService, database.DB, string) {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	base := t.TempDir()
	library := filepath.Join(base, "music")
	extractor := NewMetadataExtractor(&MetadataConfig{CoverStoragePath: filepath.Join(base, "covers")})
	songs := NewSongService(db.SongRepository(), db, extractor, NewScanner(&ScanConfig{MusicPath: library}), nil, nil)
	return songs, db, library
}

func inspectedMetadata(t *testing.T, s *SongService, id int64) *LocalFileInfo {
	t.Helper()
	file, failure := s.InspectLocalFile(context.Background(), id)
	if failure != nil {
		t.Fatal(failure)
	}
	return file
}

func addLocalMetadataTrack(t *testing.T, library, name, localTitle, album string, database database.DB) *models.Song {
	t.Helper()
	ext := filepath.Ext(name)
	data, err := os.ReadFile(filepath.Join("..", "..", "pkg", "tag", "testdata", "without_tags", "sample"+ext))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(library, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0640); err != nil {
		t.Fatal(err)
	}
	if err := tag.WriteTag(path, tag.WriteOptions{Artist: "甲 & 乙", Title: localTitle, Album: album, Genre: "Preserved"}); err != nil {
		t.Fatal(err)
	}
	song := &models.Song{Title: localTitle, Artist: "甲 & 乙", Album: album, Duration: 25,
		Type: models.TypeLocal, FilePath: path, Format: strings.TrimPrefix(ext, "."), Fingerprint: "existing", FingerprintDuration: 25}
	if err := database.SongRepository().Create(context.Background(), song); err != nil {
		t.Fatal(err)
	}
	return song
}

func audioPacketHashes(t *testing.T, path string) []byte {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is required for audio packet verification")
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "a", "-show_packets",
		"-show_data_hash", "sha256", "-show_entries", "packet=data_hash", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var hashes []byte
	for _, row := range bytes.Split(out, []byte("\n")) {
		if _, hash, ok := bytes.Cut(row, []byte("SHA256:")); ok {
			hashes = append(hashes, hash...)
		}
	}
	if len(hashes) == 0 {
		t.Fatal("missing audio packets")
	}
	return hashes
}

type rollbackMetadataTx struct{ Transactor }

func (tx rollbackMetadataTx) RunInTx(ctx context.Context, fn func(context.Context, *database.UnitOfWork) error) error {
	return tx.Transactor.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		if err := fn(ctx, uow); err != nil {
			return err
		}
		return errors.New("simulated database failure after file publication")
	})
}

func TestWriteLocalMetadataPreservesAudioAndLibraryReferences(t *testing.T) {
	ctx := context.Background()
	s, db, library := newLocalMetadataFixture(t)
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(0, 0, color.White)
	var cover bytes.Buffer
	if err := png.Encode(&cover, img); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: localMetadataTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(cover.Bytes()))}, nil
	})}
	tracks := []*models.Song{
		addLocalMetadataTrack(t, library, "parent/child/a.mp3", "測試曲", "專輯", db),
		addLocalMetadataTrack(t, library, "parent/child/b.flac", "測試曲", "專輯", db),
		addLocalMetadataTrack(t, library, "parent/child/c.m4a", "測試曲", "專輯", db),
	}
	sidecar := strings.TrimSuffix(tracks[1].FilePath, ".flac") + ".lrc"
	if err := os.WriteFile(sidecar, []byte("[00:01]旁掛歌詞"), 0600); err != nil {
		t.Fatal(err)
	}
	manual := tracks[2]
	manual.Lyric = models.LyricPayloadFromLRC("[00:01]手動歌詞").MarshalString()
	manual.LyricSource = models.LyricSourceManual
	if err := db.SongRepository().Update(ctx, manual); err != nil {
		t.Fatal(err)
	}
	if err := db.SongRepository().MarkMetadataOverrides(ctx, manual.ID, models.MetadataOverrideTitle|models.MetadataOverrideLyrics, time.Now().UnixNano()); err != nil {
		t.Fatal(err)
	}
	if n, _, err := db.PlaylistSongRepository().AddSongsBatch(ctx, 1, 0, []int64{tracks[0].ID}); err != nil || n != 1 {
		t.Fatalf("add favourite: %d, %v", n, err)
	}
	if err := db.PlayHistoryRepository().Record(ctx, "playlist", "1", tracks[0].ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	for index, old := range tracks {
		original := audioPacketHashes(t, old.FilePath)
		file := inspectedMetadata(t, s, old.ID)
		if file.LookupLyrics != (index == 0) || !file.LookupCover {
			t.Fatalf("provider eligibility: %+v", file)
		}
		relative := filepath.Join("parent", filepath.Base(old.FilePath))
		written := s.writeLocalMetadata(ctx, LocalMetadataUpdate{
			SongID: old.ID, Revision: file.Revision, TargetPath: filepath.ToSlash(relative),
			Title: "测试曲", Artists: []string{"甲", "乙"}, Album: "专辑", Track: "3/10", Year: 2021,
			Lyrics: models.LyricPayload{Lyric: "[00:01.00]测试歌词"}, CoverURL: "https://fixture.invalid/cover.png",
		}, client)
		if written.State != "written" {
			t.Fatalf("write: %+v", written)
		}
		want := filepath.Join(library, relative)
		song, err := db.SongRepository().GetByID(ctx, old.ID)
		if err != nil || song.FilePath != want || song.Track != "3/10" || song.Year != 2021 || song.Album != "专辑" {
			t.Fatalf("updated song: %+v, %v", song, err)
		}
		if !bytes.Equal(audioPacketHashes(t, want), original) {
			t.Fatalf("audio packets changed for %s", want)
		}
		f, err := os.Open(want)
		if err != nil {
			t.Fatal(err)
		}
		meta, err := tag.ReadFrom(f)
		_ = f.Close()
		if err != nil || meta.Picture() == nil || !bytes.Equal(meta.Picture().Data, cover.Bytes()) || meta.Genre() != "Preserved" {
			t.Fatalf("metadata/cover did not round trip %s: %v", want, err)
		}
		switch index {
		case 0:
			if meta.Lyrics() != "[00:01.00]测试歌词" || meta.Title() != "测试曲" {
				t.Fatalf("title/lyrics: %q / %q", meta.Title(), meta.Lyrics())
			}
		case 1:
			copied, err := os.ReadFile(filepath.Join(library, "parent", "b.lrc"))
			if err != nil || string(copied) != "[00:01]旁掛歌詞" || meta.Lyrics() != "" {
				t.Fatalf("sidecar: %q, embedded %q, %v", copied, meta.Lyrics(), err)
			}
		case 2:
			if meta.Title() != "測試曲" || song.LyricSource != models.LyricSourceManual || song.Lyric != manual.Lyric {
				t.Fatalf("manual metadata changed: %+v", song)
			}
		}
		if _, err := os.Stat(old.FilePath); !os.IsNotExist(err) {
			t.Fatalf("original path remains after moving: %v", err)
		}
	}
	history, err := db.PlayHistoryRepository().List(ctx, "playlist", "1", 10)
	if err != nil || len(history) != 1 || history[0].SongID != tracks[0].ID {
		t.Fatalf("play history: %+v, %v", history, err)
	}
	n, err := db.SongRepository().CountPlaylistsContaining(ctx, tracks[0].ID)
	if err != nil || n != 1 {
		t.Fatalf("playlist membership: %d, %v", n, err)
	}
}

func TestWriteLocalMetadataRejectsConflictsAndChanges(t *testing.T) {
	ctx := context.Background()
	s, db, library := newLocalMetadataFixture(t)
	source := addLocalMetadataTrack(t, library, "album/source.mp3", "測試曲", "專輯", db)
	occupied := addLocalMetadataTrack(t, library, "album/occupied.mp3", "Other", "專輯", db)
	cue := addLocalMetadataTrack(t, library, "album/shared.flac", "Shared", "專輯", db)
	cue.CueSourcePath = cue.FilePath + ".cue"
	if err := db.SongRepository().Update(ctx, cue); err != nil {
		t.Fatal(err)
	}
	if _, failure := s.InspectLocalFile(ctx, cue.ID); failure == nil || failure.State != "shared" {
		t.Fatalf("expected shared audio failure: %v", failure)
	}
	before, err := os.ReadFile(occupied.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	file := inspectedMetadata(t, s, source.ID)
	update := LocalMetadataUpdate{SongID: source.ID, Revision: file.Revision, TargetPath: "album/occupied.mp3", Title: "Renamed"}
	if result := s.WriteLocalMetadata(ctx, update); result.State != "conflict" {
		t.Fatalf("existing destination: %+v", result)
	}
	if after, err := os.ReadFile(occupied.FilePath); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("destination overwritten: %v", err)
	}
	updated, err := os.ReadFile(source.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	updated[len(updated)-1] ^= 1
	if err := os.WriteFile(source.FilePath, updated, 0600); err != nil {
		t.Fatal(err)
	}
	update.TargetPath = "album/new.mp3"
	if result := s.WriteLocalMetadata(ctx, update); result.State != "changed" {
		t.Fatalf("source change: %+v", result)
	}
	if after, err := os.ReadFile(source.FilePath); err != nil || !bytes.Equal(updated, after) {
		t.Fatalf("changed source overwritten: %v", err)
	}
}

func TestWriteLocalMetadataRollsBackPublication(t *testing.T) {
	ctx := context.Background()
	s, db, library := newLocalMetadataFixture(t)
	s.tx = rollbackMetadataTx{db}
	for _, target := range []string{"album/target.mp3", "album/source.mp3"} {
		var source *models.Song
		if previous, err := db.SongRepository().List(ctx, &database.SongFilter{Type: models.TypeLocal}); err == nil && len(previous) != 0 {
			source = previous[0]
		} else {
			source = addLocalMetadataTrack(t, library, "album/source.mp3", "測試曲", "專輯", db)
		}
		file := inspectedMetadata(t, s, source.ID)
		original, err := os.ReadFile(source.FilePath)
		if err != nil {
			t.Fatal(err)
		}
		got := s.WriteLocalMetadata(ctx, LocalMetadataUpdate{SongID: source.ID, Revision: file.Revision, TargetPath: target, Title: "Renamed"})
		if got.State != "error" || !strings.Contains(got.Detail, "simulated database failure") {
			t.Fatalf("expected rollback: %+v", got)
		}
		song, err := db.SongRepository().GetByID(ctx, source.ID)
		if err != nil || song.FilePath != source.FilePath || song.Title != source.Title {
			t.Fatalf("metadata after rollback: %+v %v", song, err)
		}
		if data, err := os.ReadFile(source.FilePath); err != nil || !bytes.Equal(data, original) {
			t.Fatalf("source after rollback: %v", err)
		}
	}
	if children, err := os.ReadDir(filepath.Join(library, "album")); err != nil || len(children) != 1 {
		t.Fatalf("temporary paths after rollback: %v %v", children, err)
	}
}
