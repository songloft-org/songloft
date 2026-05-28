package services

import (
	"context"
	"testing"

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

	songs, err := service.GetSongs(ctx, playlist.ID, 20, 0)
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
