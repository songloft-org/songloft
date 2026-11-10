package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func seedPlayHistorySongs(t *testing.T, svc *PlayHistoryService, n int) []*models.Song {
	t.Helper()
	songs := make([]*models.Song, 0, n)
	for i := range n {
		songs = append(songs, &models.Song{
			Type:     models.TypeLocal,
			Title:    fmt.Sprintf("歌曲 %d", i),
			Artist:   fmt.Sprintf("歌手 %d", i),
			FilePath: fmt.Sprintf("/m/p%d.mp3", i),
		})
	}
	if err := svc.db.SongRepository().BatchCreate(context.Background(), songs); err != nil {
		t.Fatalf("BatchCreate: %v", err)
	}
	return songs
}

// TestPlayHistoryServiceRecordTrimsInTx Record 在一个事务内完成 upsert + 裁剪，
// 写入超过上限后总数恰好停在 MaxPlayHistoryPerContext。
func TestPlayHistoryServiceRecordTrimsInTx(t *testing.T) {
	svc := NewPlayHistoryService(testutil.OpenMemoryDB(t))
	ctx := context.Background()
	songs := seedPlayHistorySongs(t, svc, MaxPlayHistoryPerContext+5)

	base := time.Now().Add(-time.Hour)
	for i, song := range songs {
		if err := svc.Record(ctx, models.PlayContextPlaylist, "1", song.ID, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
	}

	entries, err := svc.List(ctx, models.PlayContextPlaylist, "1", MaxPlayHistoryPerContext)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != MaxPlayHistoryPerContext {
		t.Fatalf("expected exactly %d entries, got %d", MaxPlayHistoryPerContext, len(entries))
	}
	// 最新写入的那首必须在最前面
	if entries[0].Song.ID != songs[len(songs)-1].ID {
		t.Errorf("expected newest song %d first, got %d", songs[len(songs)-1].ID, entries[0].Song.ID)
	}
}

// TestPlayHistoryServiceListHydratesSongs List 返回完整歌曲详情，
// 且顺序严格按播放时间倒序（ListByIDs 的返回顺序不保证，service 必须自己重排）。
func TestPlayHistoryServiceListHydratesSongs(t *testing.T) {
	svc := NewPlayHistoryService(testutil.OpenMemoryDB(t))
	ctx := context.Background()
	songs := seedPlayHistorySongs(t, svc, 3)
	base := time.Now().Add(-time.Hour)

	// 乱序写入：songs[1] 最新，songs[2] 次新，songs[0] 最旧
	for i, idx := range []int{0, 2, 1} {
		if err := svc.Record(ctx, "artist", "周杰伦", songs[idx].ID, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	entries, err := svc.List(ctx, "artist", "周杰伦", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []int64{songs[1].ID, songs[2].ID, songs[0].ID}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(entries))
	}
	for i, wantID := range want {
		if entries[i].Song == nil {
			t.Fatalf("entry %d has nil song", i)
		}
		if entries[i].Song.ID != wantID {
			t.Errorf("entry %d: expected song %d, got %d", i, wantID, entries[i].Song.ID)
		}
		if entries[i].Song.Title == "" || entries[i].Song.Artist == "" {
			t.Errorf("entry %d: song not hydrated: %+v", i, entries[i].Song)
		}
		if entries[i].PlayCount != 1 {
			t.Errorf("entry %d: expected play_count 1, got %d", i, entries[i].PlayCount)
		}
	}
}

// TestPlayHistoryServiceRejectsInvalidContext 非法 context_type / 空 key 一律拒绝，
// 且错误可用 errors.Is 判定以便 handler 返回 400。
func TestPlayHistoryServiceRejectsInvalidContext(t *testing.T) {
	svc := NewPlayHistoryService(testutil.OpenMemoryDB(t))
	ctx := context.Background()
	songs := seedPlayHistorySongs(t, svc, 1)

	for _, badType := range []string{"", "folder", "PLAYLIST", "keyword"} {
		err := svc.Record(ctx, badType, "1", songs[0].ID, time.Now())
		if !errors.Is(err, ErrInvalidPlayContext) {
			t.Errorf("Record(type=%q): expected ErrInvalidPlayContext, got %v", badType, err)
		}
		if _, err := svc.List(ctx, badType, "1", 50); !errors.Is(err, ErrInvalidPlayContext) {
			t.Errorf("List(type=%q): expected ErrInvalidPlayContext, got %v", badType, err)
		}
		if _, err := svc.Clear(ctx, badType, "1"); !errors.Is(err, ErrInvalidPlayContext) {
			t.Errorf("Clear(type=%q): expected ErrInvalidPlayContext, got %v", badType, err)
		}
		if err := svc.DeleteEntry(ctx, badType, "1", songs[0].ID); !errors.Is(err, ErrInvalidPlayContext) {
			t.Errorf("DeleteEntry(type=%q): expected ErrInvalidPlayContext, got %v", badType, err)
		}
	}

	if err := svc.Record(ctx, models.PlayContextPlaylist, "", songs[0].ID, time.Now()); !errors.Is(err, ErrInvalidPlayContext) {
		t.Errorf("Record(empty key): expected ErrInvalidPlayContext, got %v", err)
	}
}

// TestIsValidPlayContextType 合法上下文类型 = playlist + tag + 全部分面维度。
func TestIsValidPlayContextType(t *testing.T) {
	valid := []string{"playlist", "tag", "artist", "album", "genre", "year", "decade", "language", "style"}
	for _, ct := range valid {
		if !IsValidPlayContextType(ct) {
			t.Errorf("expected %q to be a valid play context type", ct)
		}
	}
	for _, ct := range []string{"", "folder", "keyword", "track", "isrc"} {
		if IsValidPlayContextType(ct) {
			t.Errorf("expected %q to be rejected", ct)
		}
	}
}

// TestPlaylistDeleteClearsPlayHistory 删除歌单时清理其播放历史（play_history 无外键级联到
// playlists，只能显式清理），且批量删除时绝不能连带清空 built_in 歌单的历史 ——
// 仓储层会跳过 built_in，它并没有被删掉。
func TestPlaylistDeleteClearsPlayHistory(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	ctx := context.Background()

	histSvc := NewPlayHistoryService(mdb)
	plSvc := NewPlaylistService(mdb.PlaylistRepository(), mdb.PlaylistSongRepository(), mdb.SongRepository(), nil)
	plSvc.SetPlayHistoryCleaner(mdb.PlayHistoryRepository())

	songs := seedPlayHistorySongs(t, histSvc, 1)
	songID := songs[0].ID

	// 两个普通歌单 + 迁移预置的内置歌单 id=1「收藏」
	makePlaylist := func(name string) int64 {
		p := &models.Playlist{Type: models.PlaylistTypeNormal, Name: name}
		if err := mdb.PlaylistRepository().Create(ctx, p); err != nil {
			t.Fatalf("create playlist %s: %v", name, err)
		}
		return p.ID
	}
	soloID := makePlaylist("单删测试")
	batchID := makePlaylist("批删测试")
	const builtInID int64 = 1

	for _, key := range []string{fmt.Sprint(soloID), fmt.Sprint(batchID), fmt.Sprint(builtInID)} {
		if err := histSvc.Record(ctx, models.PlayContextPlaylist, key, songID, time.Now()); err != nil {
			t.Fatalf("Record for playlist %s: %v", key, err)
		}
	}

	historyCount := func(playlistID int64) int {
		t.Helper()
		n, err := mdb.PlayHistoryRepository().Count(ctx, models.PlayContextPlaylist, fmt.Sprint(playlistID))
		if err != nil {
			t.Fatalf("Count for playlist %d: %v", playlistID, err)
		}
		return n
	}

	// 单个删除
	if err := plSvc.Delete(ctx, soloID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := historyCount(soloID); got != 0 {
		t.Errorf("deleted playlist %d: expected history cleared, got %d rows", soloID, got)
	}

	// 批量删除，故意把内置歌单混进去
	if _, err := plSvc.BatchDelete(ctx, []int64{builtInID, batchID}); err != nil {
		t.Fatalf("BatchDelete: %v", err)
	}
	if got := historyCount(batchID); got != 0 {
		t.Errorf("batch-deleted playlist %d: expected history cleared, got %d rows", batchID, got)
	}
	if got := historyCount(builtInID); got != 1 {
		t.Errorf("built-in playlist %d was not deleted, its history must survive; got %d rows", builtInID, got)
	}
}

// TestSongTagDeleteClearsPlayHistory 删除标签时清理其播放历史（与删歌单同理：
// play_history 无外键级联到 song_tags，只能显式清理），且不波及其他标签的历史。
func TestSongTagDeleteClearsPlayHistory(t *testing.T) {
	mdb := testutil.OpenMemoryDB(t)
	ctx := context.Background()

	histSvc := NewPlayHistoryService(mdb)
	tagSvc := NewSongTagService(mdb.SongTagRepository(), mdb)

	songs := seedPlayHistorySongs(t, histSvc, 1)
	songID := songs[0].ID

	makeTag := func(name string) int64 {
		id, err := mdb.SongTagRepository().Create(ctx, name, "")
		if err != nil {
			t.Fatalf("create tag %s: %v", name, err)
		}
		return id
	}
	soloID := makeTag("单删测试")
	otherID := makeTag("保留测试")

	for _, key := range []string{fmt.Sprint(soloID), fmt.Sprint(otherID)} {
		if err := histSvc.Record(ctx, models.PlayContextTag, key, songID, time.Now()); err != nil {
			t.Fatalf("Record for tag %s: %v", key, err)
		}
	}

	historyCount := func(tagID int64) int {
		t.Helper()
		n, err := mdb.PlayHistoryRepository().Count(ctx, models.PlayContextTag, fmt.Sprint(tagID))
		if err != nil {
			t.Fatalf("Count for tag %d: %v", tagID, err)
		}
		return n
	}

	if err := tagSvc.Delete(ctx, soloID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := historyCount(soloID); got != 0 {
		t.Errorf("deleted tag %d: expected history cleared, got %d rows", soloID, got)
	}
	if got := historyCount(otherID); got != 1 {
		t.Errorf("unrelated tag %d must keep its history; got %d rows", otherID, got)
	}
}

// TestPlayHistoryServiceEmptyContextReturnsEmptySlice 没有记录时返回空切片而非 nil，
// handler 直接序列化即可得到 [] 而不是 null。
func TestPlayHistoryServiceEmptyContextReturnsEmptySlice(t *testing.T) {
	svc := NewPlayHistoryService(testutil.OpenMemoryDB(t))
	entries, err := svc.List(context.Background(), "album", "不存在的专辑", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if entries == nil {
		t.Fatal("expected empty non-nil slice, got nil")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
