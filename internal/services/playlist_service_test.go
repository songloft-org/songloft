package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

// playlistTestEnv 把 :memory: SQLite 下需要的 3 个仓储打包好,
// 便于每个 PlaylistService 测试一次性获取。
type playlistTestEnv struct {
	playlists     *database.PlaylistRepository
	playlistSongs *database.PlaylistSongRepository
	songs         *database.SongRepository
}

func newPlaylistTestEnv(t *testing.T) *playlistTestEnv {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	return &playlistTestEnv{
		playlists:     mdb.PlaylistRepository(),
		playlistSongs: mdb.PlaylistSongRepository(),
		songs:         mdb.SongRepository(),
	}
}

func (e *playlistTestEnv) newService() *PlaylistService {
	return NewPlaylistService(e.playlists, e.playlistSongs, e.songs, nil)
}

func TestPlaylistServiceCreate(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	tests := []struct {
		name     string
		playlist *models.Playlist
		wantErr  bool
	}{
		{
			name: "create normal playlist",
			playlist: &models.Playlist{
				Type: models.PlaylistTypeNormal,
				Name: "我的歌单",
			},
			wantErr: false,
		},
		{
			name: "create radio playlist",
			playlist: &models.Playlist{
				Type: models.PlaylistTypeRadio,
				Name: "电台歌单",
			},
			wantErr: false,
		},
		{
			name: "invalid playlist - missing name",
			playlist: &models.Playlist{
				Type: models.PlaylistTypeNormal,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Create(ctx, tt.playlist)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlaylistServiceGetByID(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := service.GetByID(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Name != playlist.Name {
		t.Errorf("GetByID() Name = %v, want %v", got.Name, playlist.Name)
	}
}

func TestPlaylistServiceUpdate(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "原名称",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	playlist.Name = "新名称"
	if err := service.Update(ctx, playlist); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ := service.GetByID(ctx, playlist.ID)
	if got.Name != "新名称" {
		t.Errorf("Update() Name = %v, want %v", got.Name, "新名称")
	}
}

func TestPlaylistServiceDelete(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := service.Delete(ctx, playlist.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := service.GetByID(ctx, playlist.ID)
	if err == nil {
		t.Error("GetByID() should return error after deletion")
	}
}

func TestPlaylistServiceDeleteBuiltIn(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type:   models.PlaylistTypeNormal,
		Name:   "内置歌单",
		Labels: []string{models.PlaylistLabelBuiltIn},
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := service.Delete(ctx, playlist.ID); err == nil {
		t.Error("Delete() should return error for built-in playlist")
	}
}

func TestPlaylistServiceList(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	// 迁移会预置 2 条内置歌单(收藏/电台收藏)。
	baseList, err := service.List(ctx, &database.PlaylistFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	baseCount := len(baseList)

	playlists := []*models.Playlist{
		{Type: models.PlaylistTypeNormal, Name: "歌单1"},
		{Type: models.PlaylistTypeNormal, Name: "歌单2"},
	}
	for _, playlist := range playlists {
		if err := service.Create(ctx, playlist); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	list, err := service.List(ctx, &database.PlaylistFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != baseCount+2 {
		t.Errorf("List() count = %v, want %v", len(list), baseCount+2)
	}
}

func TestPlaylistServiceAddSong(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}
	if err := env.songs.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	if err := service.AddSong(ctx, playlist.ID, song.ID); err != nil {
		t.Fatalf("AddSong() error = %v", err)
	}
}

func TestPlaylistServiceAddSongTypeCheck(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "普通歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	radio := &models.Song{
		Type:  models.TypeRadio,
		Title: "测试电台",
		URL:   "https://example.com/radio.m3u8",
	}
	if err := env.songs.Create(ctx, radio); err != nil {
		t.Fatalf("create radio song: %v", err)
	}

	if err := service.AddSong(ctx, playlist.ID, radio.ID); err == nil {
		t.Error("AddSong() should return error when adding radio to normal playlist")
	}
}

func TestPlaylistServiceRemoveSong(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "待移除歌曲",
		FilePath: "/music/remove.mp3",
	}
	if err := env.songs.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}
	if err := service.AddSong(ctx, playlist.ID, song.ID); err != nil {
		t.Fatalf("AddSong() error = %v", err)
	}

	if err := service.RemoveSong(ctx, playlist.ID, song.ID); err != nil {
		t.Fatalf("RemoveSong() error = %v", err)
	}
}

func TestPlaylistServiceGetSongs(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	songs, err := service.GetSongs(ctx, playlist.ID, database.PlaylistSongFilter{Limit: 20})
	if err != nil {
		t.Fatalf("GetSongs() error = %v", err)
	}
	if songs == nil {
		t.Error("GetSongs() should not return nil")
	}
}

func TestPlaylistServiceReorderSongs(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tracks := []*models.Song{
		{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"},
		{Type: models.TypeLocal, Title: "歌曲2", FilePath: "/music/2.mp3"},
		{Type: models.TypeLocal, Title: "歌曲3", FilePath: "/music/3.mp3"},
	}
	for _, song := range tracks {
		if err := env.songs.Create(ctx, song); err != nil {
			t.Fatalf("create song: %v", err)
		}
		if err := service.AddSong(ctx, playlist.ID, song.ID); err != nil {
			t.Fatalf("AddSong() error = %v", err)
		}
	}

	songIDs := []int64{tracks[2].ID, tracks[0].ID, tracks[1].ID}
	if err := service.ReorderSongs(ctx, playlist.ID, songIDs); err != nil {
		t.Fatalf("ReorderSongs() error = %v", err)
	}
}

func TestPlaylistServiceReorderSongsMismatch(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tracks := []*models.Song{
		{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"},
		{Type: models.TypeLocal, Title: "歌曲2", FilePath: "/music/2.mp3"},
	}
	for _, song := range tracks {
		if err := env.songs.Create(ctx, song); err != nil {
			t.Fatalf("create song: %v", err)
		}
		if err := service.AddSong(ctx, playlist.ID, song.ID); err != nil {
			t.Fatalf("AddSong() error = %v", err)
		}
	}

	songIDs := []int64{tracks[0].ID}
	if err := service.ReorderSongs(ctx, playlist.ID, songIDs); err == nil {
		t.Error("ReorderSongs() should return error when song count mismatch")
	}
}

func TestPlaylistServiceSortSongs(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tracks := []*models.Song{
		{Type: models.TypeLocal, Title: "03. Third", FilePath: "/music/3.mp3"},
		{Type: models.TypeLocal, Title: "01. First", FilePath: "/music/1.mp3"},
		{Type: models.TypeLocal, Title: "02. Second", FilePath: "/music/2.mp3"},
	}
	for _, song := range tracks {
		if err := env.songs.Create(ctx, song); err != nil {
			t.Fatalf("create song: %v", err)
		}
		if err := service.AddSong(ctx, playlist.ID, song.ID); err != nil {
			t.Fatalf("AddSong() error = %v", err)
		}
	}

	// name_asc: 01. First, 02. Second, 03. Third
	t.Run("name_asc", func(t *testing.T) {
		if err := service.SortSongs(ctx, playlist.ID, "name_asc"); err != nil {
			t.Fatalf("SortSongs(name_asc) error = %v", err)
		}
		songs, _ := service.GetSongs(ctx, playlist.ID, database.PlaylistSongFilter{Limit: 100})
		if songs[0].Title != "01. First" || songs[1].Title != "02. Second" || songs[2].Title != "03. Third" {
			t.Errorf("name_asc order wrong: %v, %v, %v", songs[0].Title, songs[1].Title, songs[2].Title)
		}
	})

	// name_desc: 03. Third, 02. Second, 01. First
	t.Run("name_desc", func(t *testing.T) {
		if err := service.SortSongs(ctx, playlist.ID, "name_desc"); err != nil {
			t.Fatalf("SortSongs(name_desc) error = %v", err)
		}
		songs, _ := service.GetSongs(ctx, playlist.ID, database.PlaylistSongFilter{Limit: 100})
		if songs[0].Title != "03. Third" || songs[1].Title != "02. Second" || songs[2].Title != "01. First" {
			t.Errorf("name_desc order wrong: %v, %v, %v", songs[0].Title, songs[1].Title, songs[2].Title)
		}
	})

	// number_prefix: 01. First, 02. Second, 03. Third
	t.Run("number_prefix", func(t *testing.T) {
		if err := service.SortSongs(ctx, playlist.ID, "number_prefix"); err != nil {
			t.Fatalf("SortSongs(number_prefix) error = %v", err)
		}
		songs, _ := service.GetSongs(ctx, playlist.ID, database.PlaylistSongFilter{Limit: 100})
		if songs[0].Title != "01. First" || songs[1].Title != "02. Second" || songs[2].Title != "03. Third" {
			t.Errorf("number_prefix order wrong: %v, %v, %v", songs[0].Title, songs[1].Title, songs[2].Title)
		}
	})

	// shuffle: 3 songs should all be present
	t.Run("shuffle", func(t *testing.T) {
		if err := service.SortSongs(ctx, playlist.ID, "shuffle"); err != nil {
			t.Fatalf("SortSongs(shuffle) error = %v", err)
		}
		songs, _ := service.GetSongs(ctx, playlist.ID, database.PlaylistSongFilter{Limit: 100})
		if len(songs) != 3 {
			t.Errorf("shuffle should keep 3 songs, got %d", len(songs))
		}
	})

	// invalid action
	t.Run("invalid_action", func(t *testing.T) {
		if err := service.SortSongs(ctx, playlist.ID, "bad_action"); err == nil {
			t.Error("SortSongs() should return error for invalid action")
		}
	})
}

func TestPlaylistServiceSortSongsPinyin(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()
	playlist := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "pinyin"}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"夜曲", "beta", "阿尔法", "apple", "北京"} {
		song := &models.Song{Type: models.TypeLocal, Title: title, FilePath: fmt.Sprintf("/pinyin/%d.mp3", i)}
		if err := env.songs.Create(ctx, song); err != nil {
			t.Fatal(err)
		}
		if err := service.AddSong(ctx, playlist.ID, song.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, action := range []string{"name_asc", "name_desc"} {
		if err := service.SortSongs(ctx, playlist.ID, action); err != nil {
			t.Fatal(err)
		}
		songs, err := service.GetSongs(ctx, playlist.ID, database.PlaylistSongFilter{})
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, song := range songs {
			got = append(got, song.Title)
		}
		want := []string{"阿尔法", "apple", "北京", "beta", "夜曲"}
		if action == "name_desc" {
			slices.Reverse(want)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s persisted %v, want %v", action, got, want)
		}
	}
}

func TestPlaylistServiceSortSongsBuiltIn(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type:   models.PlaylistTypeNormal,
		Name:   "内置歌单",
		Labels: []string{models.PlaylistLabelBuiltIn},
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := service.SortSongs(ctx, playlist.ID, "name_asc"); err != nil {
		t.Errorf("SortSongs() should allow empty built-in playlist: %v", err)
	}
}

func TestPlaylistServiceUpdateInvalid(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		ID:   999,
		Type: models.PlaylistTypeNormal,
		Name: "不存在的歌单",
	}
	if err := service.Update(ctx, playlist); err == nil {
		t.Error("Update() should return error for non-existent playlist")
	}
}

func TestPlaylistServiceUpdateInvalidData(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	playlist.Name = ""
	if err := service.Update(ctx, playlist); err == nil {
		t.Error("Update() should return error for invalid data")
	}
}

func TestPlaylistServiceAddSongsBatch(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "批量歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 准备 5 首本地歌 + 1 首电台歌 + 1 个不存在的 ID。
	localIDs := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		s := &models.Song{Type: models.TypeLocal, Title: "歌", FilePath: "/m/" + string(rune('a'+i)) + ".mp3"}
		if err := env.songs.Create(ctx, s); err != nil {
			t.Fatalf("create song: %v", err)
		}
		localIDs = append(localIDs, s.ID)
	}
	radio := &models.Song{Type: models.TypeRadio, Title: "电台", URL: "https://e.com/r.m3u8"}
	if err := env.songs.Create(ctx, radio); err != nil {
		t.Fatalf("create radio: %v", err)
	}

	all := append([]int64{}, localIDs...)
	all = append(all, radio.ID, 99999)
	added, skipped, err := service.AddSongs(ctx, playlist.ID, all)
	if err != nil {
		t.Fatalf("AddSongs() error = %v", err)
	}
	if added != 5 {
		t.Errorf("added = %d, want 5", added)
	}
	if skipped != 2 {
		t.Errorf("skipped = %d, want 2 (radio + missing)", skipped)
	}

	// 二次添加同一批应全部 skipped（已存在）。
	added2, skipped2, err := service.AddSongs(ctx, playlist.ID, localIDs)
	if err != nil {
		t.Fatalf("AddSongs() second call error = %v", err)
	}
	if added2 != 0 {
		t.Errorf("added2 = %d, want 0", added2)
	}
	if skipped2 != 5 {
		t.Errorf("skipped2 = %d, want 5", skipped2)
	}

	// 验证 position 连续递增（从 1 起，无空缺）。
	songs, err := service.GetSongs(ctx, playlist.ID, database.PlaylistSongFilter{Limit: 100})
	if err != nil {
		t.Fatalf("GetSongs() error = %v", err)
	}
	if len(songs) != 5 {
		t.Errorf("playlist size = %d, want 5", len(songs))
	}
}

func TestPlaylistServiceAddSongsBatchEmpty(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "空"}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	added, skipped, err := service.AddSongs(ctx, playlist.ID, nil)
	if err != nil || added != 0 || skipped != 0 {
		t.Errorf("AddSongs(nil) = (%d, %d, %v), want (0, 0, nil)", added, skipped, err)
	}
}

// TestReorderPlaylistsPartial 覆盖 songloft#266：隐藏总目录歌单后，前端只发可见歌单子集，
// 重排不应报「count mismatch」，且隐藏歌单应保持原位。
func TestReorderPlaylistsPartial(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	// A(可见) → H(隐藏) → B(可见)，按创建顺序 position 递增。
	pA := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "A"}
	pH := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "H", Labels: []string{models.PlaylistLabelHidden}}
	pB := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "B"}
	for _, p := range []*models.Playlist{pA, pH, pB} {
		if err := service.Create(ctx, p); err != nil {
			t.Fatalf("Create(%s) error = %v", p.Name, err)
		}
	}

	// 记录重排前隐藏歌单 H 所处的槽位下标（全量列表按 position 升序）。
	slotOf := func(id int64) int {
		all, err := env.playlists.List(ctx, &database.PlaylistFilter{Limit: 0})
		if err != nil {
			t.Fatalf("List(all) error = %v", err)
		}
		for i, p := range all {
			if p.ID == id {
				return i
			}
		}
		t.Fatalf("playlist %d not found in list", id)
		return -1
	}
	hSlotBefore := slotOf(pH.ID)

	// 模拟前端：拿到排除隐藏后的可见歌单（按 position 升序），再反转发给后端。
	visible, err := env.playlists.List(ctx, &database.PlaylistFilter{
		ExcludeLabels: []string{models.PlaylistLabelHidden},
		Limit:         0,
	})
	if err != nil {
		t.Fatalf("List(visible) error = %v", err)
	}
	reversed := make([]int64, len(visible))
	for i, p := range visible {
		reversed[len(visible)-1-i] = p.ID
	}

	// 修复前这里会因 count mismatch 报错（可见子集数量 != 全量歌单数）。
	if err := service.ReorderPlaylists(ctx, reversed); err != nil {
		t.Fatalf("ReorderPlaylists() error = %v", err)
	}

	all, err := env.playlists.List(ctx, &database.PlaylistFilter{Limit: 0})
	if err != nil {
		t.Fatalf("List(all) error = %v", err)
	}
	posOf := make(map[int64]int, len(all))
	for i, p := range all {
		posOf[p.ID] = i
	}
	// 可见歌单按新顺序排列：B 现在排在 A 之前。
	if posOf[pB.ID] >= posOf[pA.ID] {
		t.Errorf("expected B before A after reorder, got B=%d A=%d", posOf[pB.ID], posOf[pA.ID])
	}
	// 隐藏歌单 H 保持原槽位下标，不受可见歌单重排影响。
	if got := posOf[pH.ID]; got != hSlotBefore {
		t.Errorf("hidden playlist H should keep its slot, before=%d after=%d", hSlotBefore, got)
	}
}

func TestReorderPlaylistsRejectsUnknownID(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	if err := service.ReorderPlaylists(ctx, []int64{999999}); err == nil {
		t.Error("ReorderPlaylists() should reject unknown playlist id")
	}
}

func TestPlaylistServiceAddSongPlaylistNotFound(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	if err := service.AddSong(ctx, 999, 1); err == nil {
		t.Error("AddSong() should return error for non-existent playlist")
	}
}

func TestPlaylistServiceAddSongSongNotFound(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	if err := service.Create(ctx, playlist); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := service.AddSong(ctx, playlist.ID, 999); err == nil {
		t.Error("AddSong() should return error for non-existent song")
	}
}

// TestDeletePlaylistOrphanCleanup 验证删除歌单时的孤儿歌曲清理语义（issue #325）：
//   - 仅属于被删歌单的歌曲（remote / local）被清理；
//   - 被其他歌单或内置收藏引用的歌曲保留；
//   - 本地孤儿歌曲 deleteFiles=true 时磁盘文件被删；
//
// 复刻 handler 编排：删前收集歌单歌曲 ID → 删歌单 → DeleteOrphanSongs。
func TestDeletePlaylistOrphanCleanup(t *testing.T) {
	env := newPlaylistTestEnv(t)
	playlistSvc := env.newService()
	songSvc := NewSongService(env.songs, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// 目标歌单 A（待删）与旁证歌单 B（引用共享歌曲，保护它不被清理）。
	plA := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "待删歌单"}
	plB := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "其他歌单"}
	if err := playlistSvc.Create(ctx, plA); err != nil {
		t.Fatalf("create playlist A: %v", err)
	}
	if err := playlistSvc.Create(ctx, plB); err != nil {
		t.Fatalf("create playlist B: %v", err)
	}

	// 本地孤儿歌曲：准备一个真实临时文件，验证 deleteFiles=true 会删磁盘文件。
	localFile := filepath.Join(t.TempDir(), "orphan.mp3")
	if err := os.WriteFile(localFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	orphanRemote := &models.Song{Type: models.TypeRemote, Title: "孤儿网络歌", URL: "https://example.com/a.mp3"}
	orphanLocal := &models.Song{Type: models.TypeLocal, Title: "孤儿本地歌", FilePath: localFile}
	sharedRemote := &models.Song{Type: models.TypeRemote, Title: "共享网络歌", URL: "https://example.com/b.mp3"}
	favRemote := &models.Song{Type: models.TypeRemote, Title: "收藏网络歌", URL: "https://example.com/c.mp3"}
	for _, s := range []*models.Song{orphanRemote, orphanLocal, sharedRemote, favRemote} {
		if err := env.songs.Create(ctx, s); err != nil {
			t.Fatalf("create song %q: %v", s.Title, err)
		}
	}

	// 关联：A 含全部 4 首；B 也含 sharedRemote；内置收藏(id=1)含 favRemote。
	add := func(pid, sid int64, pos int) {
		if err := env.playlistSongs.AddSong(ctx, pid, sid, pos); err != nil {
			t.Fatalf("add song %d to playlist %d: %v", sid, pid, err)
		}
	}
	add(plA.ID, orphanRemote.ID, 1)
	add(plA.ID, orphanLocal.ID, 2)
	add(plA.ID, sharedRemote.ID, 3)
	add(plA.ID, favRemote.ID, 4)
	add(plB.ID, sharedRemote.ID, 1)
	add(1, favRemote.ID, 1) // 内置「收藏」歌单 id=1

	// —— 复刻 handler 编排 ——
	candidateIDs, err := playlistSvc.SongIDsInPlaylist(ctx, plA.ID)
	if err != nil {
		t.Fatalf("SongIDsInPlaylist: %v", err)
	}
	if len(candidateIDs) != 4 {
		t.Fatalf("candidateIDs = %d, want 4", len(candidateIDs))
	}
	if err := playlistSvc.Delete(ctx, plA.ID); err != nil {
		t.Fatalf("Delete playlist A: %v", err)
	}
	deleted, err := songSvc.DeleteOrphanSongs(ctx, candidateIDs, true)
	if err != nil {
		t.Fatalf("DeleteOrphanSongs: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted orphan songs = %d, want 2 (orphanRemote + orphanLocal)", deleted)
	}

	// 断言：孤儿被删，共享/收藏保留。
	assertGone := func(id int64, name string) {
		if _, err := songSvc.GetByID(ctx, id); err == nil {
			t.Errorf("song %q (id=%d) should be deleted", name, id)
		}
	}
	assertKept := func(id int64, name string) {
		if _, err := songSvc.GetByID(ctx, id); err != nil {
			t.Errorf("song %q (id=%d) should be kept, got err: %v", name, id, err)
		}
	}
	assertGone(orphanRemote.ID, "孤儿网络歌")
	assertGone(orphanLocal.ID, "孤儿本地歌")
	assertKept(sharedRemote.ID, "共享网络歌")
	assertKept(favRemote.ID, "收藏网络歌")

	// 本地孤儿磁盘文件应被删除。
	if _, err := os.Stat(localFile); !os.IsNotExist(err) {
		t.Errorf("local orphan file should be deleted, stat err = %v", err)
	}
}

// TestPlaylistServiceSetPinned 验证置顶/取消置顶，以及置顶歌单在 List 里始终排最前、
// 多个置顶歌单按置顶时间倒序（最近置顶在前）；内置歌单同样允许置顶（issue #40）。
func TestPlaylistServiceSetPinned(t *testing.T) {
	env := newPlaylistTestEnv(t)
	service := env.newService()
	ctx := context.Background()

	pA := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "A"}
	pB := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "B"}
	builtIn := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "内置歌单", Labels: []string{models.PlaylistLabelBuiltIn}}
	for _, p := range []*models.Playlist{pA, pB, builtIn} {
		if err := service.Create(ctx, p); err != nil {
			t.Fatalf("Create(%s) error = %v", p.Name, err)
		}
	}

	// 置顶 A，随后置顶内置歌单：内置歌单允许置顶，不受 Update 的内置保护逻辑限制。
	updatedA, err := service.SetPinned(ctx, pA.ID, true)
	if err != nil {
		t.Fatalf("SetPinned(A, true) error = %v", err)
	}
	if !updatedA.IsPinned() {
		t.Error("A should be pinned after SetPinned(true)")
	}

	time.Sleep(10 * time.Millisecond) // 确保置顶时间戳有先后差
	updatedBuiltIn, err := service.SetPinned(ctx, builtIn.ID, true)
	if err != nil {
		t.Fatalf("SetPinned(builtIn, true) error = %v", err)
	}
	if !updatedBuiltIn.IsPinned() {
		t.Error("built-in playlist should be pinnable and pinned")
	}

	list, err := service.List(ctx, &database.PlaylistFilter{Limit: 0})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) < 2 || list[0].ID != builtIn.ID || list[1].ID != pA.ID {
		t.Fatalf("expected pinned playlists first, most-recently-pinned first: got %+v", firstTwoNames(list))
	}
	if list[0].PinnedAt == nil || list[1].PinnedAt == nil {
		t.Error("pinned playlists should have non-nil PinnedAt")
	}

	// 取消置顶 A 后，A 不再排在最前，PinnedAt 变回 nil（不是零值时间戳）。
	unpinnedA, err := service.SetPinned(ctx, pA.ID, false)
	if err != nil {
		t.Fatalf("SetPinned(A, false) error = %v", err)
	}
	if unpinnedA.IsPinned() || unpinnedA.PinnedAt != nil {
		t.Error("A should not be pinned and PinnedAt should be nil after SetPinned(false)")
	}
	list, err = service.List(ctx, &database.PlaylistFilter{Limit: 0})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list[0].ID != builtIn.ID {
		t.Fatalf("expected builtIn still pinned-first after unpinning A: got %+v", firstTwoNames(list))
	}
}

func firstTwoNames(list []*models.Playlist) []string {
	names := make([]string, 0, 2)
	for i, p := range list {
		if i >= 2 {
			break
		}
		names = append(names, p.Name)
	}
	return names
}

// TestDeletePlaylistNoOrphanCleanupByDefault 验证不传 deleteSongs（candidateIDs 不收集）时，
// 仅删歌单、歌曲全部保留——即默认行为不变。
func TestDeletePlaylistNoOrphanCleanupByDefault(t *testing.T) {
	env := newPlaylistTestEnv(t)
	playlistSvc := env.newService()
	songSvc := NewSongService(env.songs, nil, nil, nil, nil, nil)
	ctx := context.Background()

	pl := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "歌单"}
	if err := playlistSvc.Create(ctx, pl); err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	song := &models.Song{Type: models.TypeRemote, Title: "网络歌", URL: "https://example.com/x.mp3"}
	if err := env.songs.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}
	if err := env.playlistSongs.AddSong(ctx, pl.ID, song.ID, 1); err != nil {
		t.Fatalf("add song: %v", err)
	}

	if err := playlistSvc.Delete(ctx, pl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// 未调用 DeleteOrphanSongs：歌曲应仍在。
	if _, err := songSvc.GetByID(ctx, song.ID); err != nil {
		t.Errorf("song should be kept when deleteSongs is off, got err: %v", err)
	}
}
