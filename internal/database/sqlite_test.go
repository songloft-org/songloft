package database

import (
	"context"
	"errors"
	"fmt"
	"songloft/internal/models"
	"testing"
)

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) DB {
	db, err := NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return db
}

// TestNewSQLiteDB 测试数据库初始化
func TestNewSQLiteDB(t *testing.T) {
	db, err := NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteDB() error = %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Error("NewSQLiteDB() returned nil")
	}
}

// TestCreateAndGetSong 测试创建和获取歌曲
func TestCreateAndGetSong(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		Artist:   "测试艺术家",
		Album:    "测试专辑",
		Duration: 180.5,
		FilePath: "/music/test.mp3",
		Format:   "mp3",
		BitRate:  320,
	}

	// 创建歌曲
	err := db.SongRepository().BatchCreate(ctx, []*models.Song{song})
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	if song.ID == 0 {
		t.Error("CreateSong() did not set ID")
	}

	// 获取歌曲
	got, err := db.SongRepository().GetByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}

	if got.Title != song.Title {
		t.Errorf("GetSongByID() Title = %v, want %v", got.Title, song.Title)
	}
	if got.Artist != song.Artist {
		t.Errorf("GetSongByID() Artist = %v, want %v", got.Artist, song.Artist)
	}
}

// TestUpdateSong 测试更新歌曲
func TestUpdateSong(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "原标题",
		FilePath: "/music/test.mp3",
	}

	// 创建歌曲
	err := db.SongRepository().BatchCreate(ctx, []*models.Song{song})
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 更新歌曲
	song.Title = "新标题"
	song.Artist = "新艺术家"
	err = db.SongRepository().Update(ctx, song)
	if err != nil {
		t.Fatalf("UpdateSong() error = %v", err)
	}

	// 验证更新
	got, err := db.SongRepository().GetByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}

	if got.Title != "新标题" {
		t.Errorf("UpdateSong() Title = %v, want %v", got.Title, "新标题")
	}
	if got.Artist != "新艺术家" {
		t.Errorf("UpdateSong() Artist = %v, want %v", got.Artist, "新艺术家")
	}
}

// TestDeleteSong 测试删除歌曲
func TestDeleteSong(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}

	// 创建歌曲
	err := db.SongRepository().BatchCreate(ctx, []*models.Song{song})
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 删除歌曲
	err = db.SongRepository().Delete(ctx, song.ID)
	if err != nil {
		t.Fatalf("DeleteSong() error = %v", err)
	}

	// 验证删除
	_, err = db.SongRepository().GetByID(ctx, song.ID)
	if err == nil {
		t.Error("GetSongByID() should return error for deleted song")
	}
}

// TestListSongs 测试列出歌曲
func TestListSongs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建多首歌曲
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"},
		{Type: models.TypeRemote, Title: "歌曲2", URL: "https://example.com/2.mp3"},
		{Type: models.TypeRadio, Title: "电台1", URL: "https://example.com/radio.m3u8", IsLive: true},
	}

	err := db.SongRepository().BatchCreate(ctx, songs)
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 测试无过滤
	filter := &SongFilter{}
	list, err := db.SongRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListSongs() error = %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ListSongs() count = %v, want %v", len(list), 3)
	}

	// 测试类型过滤
	filter = &SongFilter{Type: models.TypeLocal}
	list, err = db.SongRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListSongs() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListSongs() with type filter count = %v, want %v", len(list), 1)
	}

	// 测试关键词搜索
	filter = &SongFilter{Keyword: "歌曲"}
	list, err = db.SongRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListSongs() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListSongs() with keyword filter count = %v, want %v", len(list), 2)
	}

	// 测试分页
	filter = &SongFilter{Limit: 2, Offset: 0}
	list, err = db.SongRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListSongs() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListSongs() with pagination count = %v, want %v", len(list), 2)
	}
}

// TestCreateAndGetPlaylist 测试创建和获取歌单
func TestCreateAndGetPlaylist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	playlist := &models.Playlist{
		Type:        models.PlaylistTypeNormal,
		Name:        "我的歌单",
		Description: "测试描述",
	}

	// 创建歌单
	err := db.PlaylistRepository().Create(ctx, playlist)
	if err != nil {
		t.Fatalf("CreatePlaylist() error = %v", err)
	}

	if playlist.ID == 0 {
		t.Error("CreatePlaylist() did not set ID")
	}

	// 获取歌单
	got, err := db.PlaylistRepository().GetByID(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetPlaylistByID() error = %v", err)
	}

	if got.Name != playlist.Name {
		t.Errorf("GetPlaylistByID() Name = %v, want %v", got.Name, playlist.Name)
	}
}

// TestCreatePlaylistNameConflict 验证同名歌单的查重逻辑（不区分类型）
func TestCreatePlaylistNameConflict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	first := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "重复名"}
	if err := db.PlaylistRepository().Create(ctx, first); err != nil {
		t.Fatalf("first CreatePlaylist() error = %v", err)
	}

	// 同名同类型 → 必须报 ErrPlaylistNameConflict
	dup := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "重复名"}
	err := db.PlaylistRepository().Create(ctx, dup)
	if !errors.Is(err, models.ErrPlaylistNameConflict) {
		t.Fatalf("expected ErrPlaylistNameConflict, got %v", err)
	}
	if dup.ID != 0 {
		t.Errorf("dup.ID should remain 0 on conflict, got %d", dup.ID)
	}

	// 同名但不同类型 → 也必须冲突
	radio := &models.Playlist{Type: models.PlaylistTypeRadio, Name: "重复名"}
	err = db.PlaylistRepository().Create(ctx, radio)
	if !errors.Is(err, models.ErrPlaylistNameConflict) {
		t.Fatalf("different type same name should also conflict, got %v", err)
	}
	if radio.ID != 0 {
		t.Errorf("radio.ID should remain 0 on conflict, got %d", radio.ID)
	}
}

// TestUpdatePlaylistNameConflict 验证改名时撞到其他歌单同名报错
func TestUpdatePlaylistNameConflict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	a := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "A"}
	if err := db.PlaylistRepository().Create(ctx, a); err != nil {
		t.Fatalf("create A error = %v", err)
	}
	b := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "B"}
	if err := db.PlaylistRepository().Create(ctx, b); err != nil {
		t.Fatalf("create B error = %v", err)
	}

	// 把 B 改成 A → 冲突
	b.Name = "A"
	err := db.PlaylistRepository().Update(ctx, b)
	if !errors.Is(err, models.ErrPlaylistNameConflict) {
		t.Fatalf("expected ErrPlaylistNameConflict on rename, got %v", err)
	}

	// 把 A 改成 A (改自己) → 允许
	a.Description = "更新描述"
	if err := db.PlaylistRepository().Update(ctx, a); err != nil {
		t.Errorf("update self should not conflict, got %v", err)
	}
}

// TestAutoCreatePlaylistsAvoidsManualNameConflict 验证自动创建撞到用户手动建的同名歌单时,
// 通过加 " (自动)" 后缀消歧,而不是直接 INSERT 出两条同名记录。
func TestAutoCreatePlaylistsAvoidsManualNameConflict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 用户先手动建一个名叫 "Pop" 的歌单(走 CreatePlaylist,会留下来)
	manual := &models.Playlist{Type: models.PlaylistTypeNormal, Name: "Pop"}
	if err := db.PlaylistRepository().Create(ctx, manual); err != nil {
		t.Fatalf("manual CreatePlaylist error = %v", err)
	}

	// 准备两首歌都在 /music/Pop 目录下(auto-create 算出的目录名就是 "Pop")
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "歌1", FilePath: "/music/Pop/1.mp3"},
		{Type: models.TypeLocal, Title: "歌2", FilePath: "/music/Pop/2.mp3"},
	}
	if err := db.SongRepository().BatchCreate(ctx, songs); err != nil {
		t.Fatalf("BatchCreateSongs error = %v", err)
	}

	resp, err := db.PlaylistRepository().AutoCreate(ctx, false)
	if err != nil {
		t.Fatalf("AutoCreatePlaylists error = %v", err)
	}
	if len(resp.Playlists) != 1 {
		t.Fatalf("expected 1 auto-created playlist, got %d", len(resp.Playlists))
	}
	autoName := resp.Playlists[0].Name
	if autoName == "Pop" {
		t.Fatalf("auto-created playlist should not reuse manual name %q", autoName)
	}
	if autoName != "Pop (自动)" {
		t.Errorf("expected disambiguated name %q, got %q", "Pop (自动)", autoName)
	}

	// 再跑一次:旧的 auto_created 会被 DELETE,新建仍然应消歧成相同后缀,不应递增到 (自动 2)
	resp2, err := db.PlaylistRepository().AutoCreate(ctx, false)
	if err != nil {
		t.Fatalf("second AutoCreatePlaylists error = %v", err)
	}
	if len(resp2.Playlists) != 1 {
		t.Fatalf("expected 1 playlist on rerun, got %d", len(resp2.Playlists))
	}
	if resp2.Playlists[0].Name != "Pop (自动)" {
		t.Errorf("rerun should produce stable name %q, got %q", "Pop (自动)", resp2.Playlists[0].Name)
	}

	// 用户手动建的 "Pop" 仍然存在,且只有一条
	pls, err := db.PlaylistRepository().List(ctx, &PlaylistFilter{Keyword: "Pop"})
	if err != nil {
		t.Fatalf("ListPlaylists error = %v", err)
	}
	popCount := 0
	for _, p := range pls {
		if p.Name == "Pop" {
			popCount++
		}
	}
	if popCount != 1 {
		t.Errorf("expected exactly 1 playlist named %q, got %d", "Pop", popCount)
	}
}

// TestAddAndRemoveSongToPlaylist 测试添加和移除歌曲到歌单
func TestAddAndRemoveSongToPlaylist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建歌单
	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	err := db.PlaylistRepository().Create(ctx, playlist)
	if err != nil {
		t.Fatalf("CreatePlaylist() error = %v", err)
	}

	// 创建歌曲
	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}
	err = db.SongRepository().BatchCreate(ctx, []*models.Song{song})
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 添加歌曲到歌单
	err = db.PlaylistSongRepository().AddSong(ctx, playlist.ID, song.ID, 1)
	if err != nil {
		t.Fatalf("AddSongToPlaylist() error = %v", err)
	}

	// 获取歌单歌曲
	songs, err := db.PlaylistSongRepository().GetSongs(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetPlaylistSongs() error = %v", err)
	}
	if len(songs) != 1 {
		t.Errorf("GetPlaylistSongs() count = %v, want %v", len(songs), 1)
	}

	// 移除歌曲
	err = db.PlaylistSongRepository().RemoveSong(ctx, playlist.ID, song.ID)
	if err != nil {
		t.Fatalf("RemoveSongFromPlaylist() error = %v", err)
	}

	// 验证移除
	songs, err = db.PlaylistSongRepository().GetSongs(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetPlaylistSongs() error = %v", err)
	}
	if len(songs) != 0 {
		t.Errorf("GetPlaylistSongs() after remove count = %v, want %v", len(songs), 0)
	}
}

// TestGetAndSetConfig 测试配置读写
func TestGetAndSetConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	config := &models.Config{
		Key:   "test_key",
		Value: `{"path": "music"}`,
	}

	// 设置配置
	err := db.ConfigRepository().Set(ctx, config)
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	// 获取配置
	got, err := db.ConfigRepository().Get(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if got.Value != config.Value {
		t.Errorf("GetConfig() Value = %v, want %v", got.Value, config.Value)
	}

	// 更新配置
	config.Value = `{"path": "new_music"}`
	err = db.ConfigRepository().Set(ctx, config)
	if err != nil {
		t.Fatalf("SetConfig() update error = %v", err)
	}

	// 验证更新
	got, err = db.ConfigRepository().Get(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if got.Value != `{"path": "new_music"}` {
		t.Errorf("GetConfig() after update Value = %v, want %v", got.Value, `{"path": "new_music"}`)
	}
}

// TestTransaction 测试事务（UnitOfWork）
func TestTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 回滚场景：返回 error 让 RunInTx 自动回滚
	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "事务测试",
		FilePath: "/music/test.mp3",
	}
	rollbackErr := fmt.Errorf("force rollback")
	err := db.RunInTx(ctx, func(ctx context.Context, uow *UnitOfWork) error {
		if err := uow.Songs.Create(ctx, song); err != nil {
			return err
		}
		return rollbackErr
	})
	if err != rollbackErr {
		t.Fatalf("RunInTx() expect rollback error, got %v", err)
	}

	if _, err := db.SongRepository().GetByID(ctx, song.ID); err == nil {
		t.Error("GetByID() should return error after rollback")
	}

	// 提交场景
	song2 := &models.Song{
		Type:     models.TypeLocal,
		Title:    "提交测试",
		FilePath: "/music/test2.mp3",
	}
	if err := db.RunInTx(ctx, func(ctx context.Context, uow *UnitOfWork) error {
		return uow.Songs.Create(ctx, song2)
	}); err != nil {
		t.Fatalf("RunInTx() commit error = %v", err)
	}

	got, err := db.SongRepository().GetByID(ctx, song2.ID)
	if err != nil {
		t.Fatalf("GetByID() after commit error = %v", err)
	}
	if got.Title != song2.Title {
		t.Errorf("GetByID() after commit Title = %v, want %v", got.Title, song2.Title)
	}
}

// TestCountSongs 测试统计歌曲数量
func TestCountSongs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建多首歌曲
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "本地歌曲1", FilePath: "/music/1.mp3"},
		{Type: models.TypeLocal, Title: "本地歌曲2", FilePath: "/music/2.mp3"},
		{Type: models.TypeRemote, Title: "网络歌曲1", URL: "https://example.com/1.mp3"},
		{Type: models.TypeRadio, Title: "电台1", URL: "https://example.com/radio.m3u8", IsLive: true},
	}

	err := db.SongRepository().BatchCreate(ctx, songs)
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 测试无过滤条件的计数
	count, err := db.SongRepository().Count(ctx, &SongFilter{})
	if err != nil {
		t.Fatalf("CountSongs() error = %v", err)
	}
	if count != 4 {
		t.Errorf("CountSongs() = %v, want %v", count, 4)
	}

	// 测试带类型过滤的计数
	count, err = db.SongRepository().Count(ctx, &SongFilter{Type: models.TypeLocal})
	if err != nil {
		t.Fatalf("CountSongs() with type filter error = %v", err)
	}
	if count != 2 {
		t.Errorf("CountSongs() with type filter = %v, want %v", count, 2)
	}

	// 测试带关键词过滤的计数
	count, err = db.SongRepository().Count(ctx, &SongFilter{Keyword: "本地"})
	if err != nil {
		t.Fatalf("CountSongs() with keyword filter error = %v", err)
	}
	if count != 2 {
		t.Errorf("CountSongs() with keyword filter = %v, want %v", count, 2)
	}
}

// TestUpdatePlaylist 测试更新歌单
func TestUpdatePlaylist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	playlist := &models.Playlist{
		Type:        models.PlaylistTypeNormal,
		Name:        "原名称",
		Description: "原描述",
	}

	// 创建歌单
	err := db.PlaylistRepository().Create(ctx, playlist)
	if err != nil {
		t.Fatalf("CreatePlaylist() error = %v", err)
	}

	// 更新歌单
	playlist.Name = "新名称"
	playlist.Description = "新描述"
	err = db.PlaylistRepository().Update(ctx, playlist)
	if err != nil {
		t.Fatalf("UpdatePlaylist() error = %v", err)
	}

	// 验证更新
	got, err := db.PlaylistRepository().GetByID(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetPlaylistByID() error = %v", err)
	}

	if got.Name != "新名称" {
		t.Errorf("UpdatePlaylist() Name = %v, want %v", got.Name, "新名称")
	}
	if got.Description != "新描述" {
		t.Errorf("UpdatePlaylist() Description = %v, want %v", got.Description, "新描述")
	}
}

// TestDeletePlaylist 测试删除歌单
func TestDeletePlaylist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}

	// 创建歌单
	err := db.PlaylistRepository().Create(ctx, playlist)
	if err != nil {
		t.Fatalf("CreatePlaylist() error = %v", err)
	}

	// 删除歌单
	err = db.PlaylistRepository().Delete(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("DeletePlaylist() error = %v", err)
	}

	// 验证删除
	_, err = db.PlaylistRepository().GetByID(ctx, playlist.ID)
	if err == nil {
		t.Error("GetPlaylistByID() should return error for deleted playlist")
	}
}

// TestListPlaylists 测试列出歌单
func TestListPlaylists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建多个歌单
	playlists := []*models.Playlist{
		{Type: models.PlaylistTypeNormal, Name: "普通歌单1", Description: "描述1", CoverURL: "https://example.com/cover1.jpg"},
		{Type: models.PlaylistTypeNormal, Name: "普通歌单2", Description: "描述2", CoverURL: "https://example.com/cover2.jpg"},
		{Type: models.PlaylistTypeRadio, Name: "电台歌单1", Description: "电台描述", CoverURL: "https://example.com/cover3.jpg"},
	}

	for _, playlist := range playlists {
		err := db.PlaylistRepository().Create(ctx, playlist)
		if err != nil {
			t.Fatalf("CreatePlaylist() error = %v", err)
		}
	}

	// 测试无过滤
	filter := &PlaylistFilter{}
	list, err := db.PlaylistRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListPlaylists() error = %v", err)
	}
	// 注意：数据库初始化时会创建2个内置歌单
	if len(list) < 3 {
		t.Errorf("ListPlaylists() count = %v, want at least %v", len(list), 3)
	}

	// 测试类型过滤
	filter = &PlaylistFilter{Type: models.PlaylistTypeNormal}
	list, err = db.PlaylistRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListPlaylists() with type filter error = %v", err)
	}
	if len(list) < 2 {
		t.Errorf("ListPlaylists() with type filter count = %v, want at least %v", len(list), 2)
	}

	// 测试关键词搜索
	filter = &PlaylistFilter{Keyword: "普通"}
	list, err = db.PlaylistRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListPlaylists() with keyword filter error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListPlaylists() with keyword filter count = %v, want %v", len(list), 2)
	}

	// 测试分页
	filter = &PlaylistFilter{Limit: 2, Offset: 0}
	list, err = db.PlaylistRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListPlaylists() with pagination error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListPlaylists() with pagination count = %v, want %v", len(list), 2)
	}

	// 测试内置歌单过滤（使用 labels 过滤）
	filter = &PlaylistFilter{Labels: []string{"built_in"}}
	list, err = db.PlaylistRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListPlaylists() with labels filter error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListPlaylists() with labels filter count = %v, want %v", len(list), 2)
	}
}

// TestDeleteConfig 测试删除配置
func TestDeleteConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	config := &models.Config{
		Key:   "test_delete_key",
		Value: `{"test": "value"}`,
	}

	// 设置配置
	err := db.ConfigRepository().Set(ctx, config)
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	// 删除配置
	err = db.ConfigRepository().Delete(ctx, "test_delete_key")
	if err != nil {
		t.Fatalf("DeleteConfig() error = %v", err)
	}

	// 验证删除
	_, err = db.ConfigRepository().Get(ctx, "test_delete_key")
	if err == nil {
		t.Error("GetConfig() should return error for deleted config")
	}
}

// TestGetSongByIDNotFound 测试获取不存在的歌曲
func TestGetSongByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	_, err := db.SongRepository().GetByID(ctx, 99999)
	if err == nil {
		t.Error("GetSongByID() should return error for non-existent song")
	}
}

// TestUpdateSongNotFound 测试更新不存在的歌曲
func TestUpdateSongNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	song := &models.Song{
		ID:       99999,
		Type:     models.TypeLocal,
		Title:    "不存在的歌曲",
		FilePath: "/music/test.mp3",
	}

	err := db.SongRepository().Update(ctx, song)
	if err == nil {
		t.Error("UpdateSong() should return error for non-existent song")
	}
}

// TestDeleteSongNotFound 测试删除不存在的歌曲
func TestDeleteSongNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := db.SongRepository().Delete(ctx, 99999)
	if err == nil {
		t.Error("DeleteSong() should return error for non-existent song")
	}
}

// TestGetPlaylistByIDNotFound 测试获取不存在的歌单
func TestGetPlaylistByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	_, err := db.PlaylistRepository().GetByID(ctx, 99999)
	if err == nil {
		t.Error("GetPlaylistByID() should return error for non-existent playlist")
	}
}

// TestUpdatePlaylistNotFound 测试更新不存在的歌单
func TestUpdatePlaylistNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	playlist := &models.Playlist{
		ID:   99999,
		Type: models.PlaylistTypeNormal,
		Name: "不存在的歌单",
	}

	err := db.PlaylistRepository().Update(ctx, playlist)
	if err == nil {
		t.Error("UpdatePlaylist() should return error for non-existent playlist")
	}
}

// TestDeletePlaylistNotFound 测试删除不存在的歌单
func TestDeletePlaylistNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := db.PlaylistRepository().Delete(ctx, 99999)
	if err == nil {
		t.Error("DeletePlaylist() should return error for non-existent playlist")
	}
}

// TestRemoveSongFromPlaylistNotFound 测试移除不存在的歌曲
func TestRemoveSongFromPlaylistNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建歌单
	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	db.PlaylistRepository().Create(ctx, playlist)

	err := db.PlaylistSongRepository().RemoveSong(ctx, playlist.ID, 99999)
	if err == nil {
		t.Error("RemoveSongFromPlaylist() should return error for non-existent song")
	}
}

// TestGetConfigNotFound 测试获取不存在的配置
func TestGetConfigNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	_, err := db.ConfigRepository().Get(ctx, "non_existent_key")
	if err == nil {
		t.Error("GetConfig() should return error for non-existent config")
	}
}

// TestDeleteConfigNotFound 测试删除不存在的配置
func TestDeleteConfigNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := db.ConfigRepository().Delete(ctx, "non_existent_key")
	if err == nil {
		t.Error("DeleteConfig() should return error for non-existent config")
	}
}

// TestListSongsWithOrdering 测试歌曲列表排序
func TestListSongsWithOrdering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建多首歌曲
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "C 歌曲", FilePath: "/music/c.mp3"},
		{Type: models.TypeLocal, Title: "A 歌曲", FilePath: "/music/a.mp3"},
		{Type: models.TypeLocal, Title: "B 歌曲", FilePath: "/music/b.mp3"},
	}

	err := db.SongRepository().BatchCreate(ctx, songs)
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 测试按标题升序排序
	filter := &SongFilter{OrderBy: "title", Order: "ASC"}
	list, err := db.SongRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListSongs() with ordering error = %v", err)
	}
	if len(list) >= 2 && list[0].Title > list[1].Title {
		t.Errorf("ListSongs() not properly ordered by title ASC")
	}

	// 测试按标题降序排序
	filter = &SongFilter{OrderBy: "title", Order: "DESC"}
	list, err = db.SongRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListSongs() with DESC ordering error = %v", err)
	}
	if len(list) >= 2 && list[0].Title < list[1].Title {
		t.Errorf("ListSongs() not properly ordered by title DESC")
	}
}

// TestCascadeDelete 测试级联删除
func TestCascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建歌单和歌曲
	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	db.PlaylistRepository().Create(ctx, playlist)

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}
	err := db.SongRepository().BatchCreate(ctx, []*models.Song{song})
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 添加歌曲到歌单
	db.PlaylistSongRepository().AddSong(ctx, playlist.ID, song.ID, 1)

	// 删除歌单
	err = db.PlaylistRepository().Delete(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("DeletePlaylist() error = %v", err)
	}

	// 验证歌曲仍然存在（只删除了关联关系）
	_, err = db.SongRepository().GetByID(ctx, song.ID)
	if err != nil {
		t.Error("Song should still exist after playlist deletion")
	}
}

// TestNewSQLiteDBWithInvalidPath 测试使用无效路径创建数据库
func TestNewSQLiteDBWithInvalidPath(t *testing.T) {
	// 使用无效的数据库路径
	_, err := NewSQLiteDB("/invalid/path/that/does/not/exist/test.db")
	if err == nil {
		t.Error("NewSQLiteDB() with invalid path should return error")
	}
}

// TestTransactionRollbackOnError 事务错误时回滚（UnitOfWork）
func TestTransactionRollbackOnError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "事务测试歌曲",
		FilePath: "/music/tx_test.mp3",
	}
	rollbackErr := fmt.Errorf("force rollback")
	err := db.RunInTx(ctx, func(ctx context.Context, uow *UnitOfWork) error {
		if err := uow.Songs.Create(ctx, song); err != nil {
			return err
		}
		return rollbackErr
	})
	if err != rollbackErr {
		t.Fatalf("RunInTx() expect rollback error, got %v", err)
	}

	if _, err := db.SongRepository().GetByID(ctx, song.ID); err == nil {
		t.Error("Song should not exist after transaction rollback")
	}
}

// TestGetSongByIDInTransaction 事务中读歌曲
func TestGetSongByIDInTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}
	if err := db.SongRepository().BatchCreate(ctx, []*models.Song{song}); err != nil {
		t.Fatalf("BatchCreate() error = %v", err)
	}

	if err := db.RunInTx(ctx, func(ctx context.Context, uow *UnitOfWork) error {
		got, err := uow.Songs.GetByID(ctx, song.ID)
		if err != nil {
			return err
		}
		if got.Title != song.Title {
			t.Errorf("GetByID() in tx Title = %v, want %v", got.Title, song.Title)
		}
		return nil
	}); err != nil {
		t.Fatalf("RunInTx() error = %v", err)
	}
}

// TestUpdateSongInTransaction 事务中改歌曲
func TestUpdateSongInTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "原标题",
		FilePath: "/music/test.mp3",
	}
	if err := db.SongRepository().BatchCreate(ctx, []*models.Song{song}); err != nil {
		t.Fatalf("BatchCreate() error = %v", err)
	}

	song.Title = "新标题"
	if err := db.RunInTx(ctx, func(ctx context.Context, uow *UnitOfWork) error {
		return uow.Songs.Update(ctx, song)
	}); err != nil {
		t.Fatalf("RunInTx() error = %v", err)
	}

	got, err := db.SongRepository().GetByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Title != "新标题" {
		t.Errorf("Update() in tx Title = %v, want %v", got.Title, "新标题")
	}
}

// TestDeleteSongInTransaction 事务中删歌曲
func TestDeleteSongInTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "待删除歌曲",
		FilePath: "/music/test.mp3",
	}
	if err := db.SongRepository().BatchCreate(ctx, []*models.Song{song}); err != nil {
		t.Fatalf("BatchCreate() error = %v", err)
	}

	if err := db.RunInTx(ctx, func(ctx context.Context, uow *UnitOfWork) error {
		return uow.Songs.Delete(ctx, song.ID)
	}); err != nil {
		t.Fatalf("RunInTx() error = %v", err)
	}

	if _, err := db.SongRepository().GetByID(ctx, song.ID); err == nil {
		t.Error("Song should not exist after deletion in transaction")
	}
}

// TestListSongsWithMultipleFilters 测试多个过滤条件组合
func TestListSongsWithMultipleFilters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建多首歌曲
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "本地歌曲 A", Artist: "艺术家 A", FilePath: "/music/a.mp3"},
		{Type: models.TypeLocal, Title: "本地歌曲 B", Artist: "艺术家 B", FilePath: "/music/b.mp3"},
		{Type: models.TypeRemote, Title: "网络歌曲 A", Artist: "艺术家 A", URL: "https://example.com/a.mp3"},
	}

	err := db.SongRepository().BatchCreate(ctx, songs)
	if err != nil {
		t.Fatalf("BatchCreateSongs() error = %v", err)
	}

	// 验证数据已插入
	allList, _ := db.SongRepository().List(ctx, &SongFilter{})
	t.Logf("Total songs inserted: %d", len(allList))
	for i, s := range allList {
		t.Logf("Song %d: Title=%s, Artist=%s, Type=%s", i+1, s.Title, s.Artist, s.Type)
	}

	// 测试类型 + 关键词组合过滤
	filter := &SongFilter{
		Type:    models.TypeLocal,
		Keyword: "艺术家 A",
	}
	list, err := db.SongRepository().List(ctx, filter)
	if err != nil {
		t.Fatalf("ListSongs() with combined filters error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListSongs() with combined filters count = %v, want %v", len(list), 1)
	}
}

// TestGetPlaylistSongsEmpty 测试获取空歌单的歌曲列表
func TestGetPlaylistSongsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建空歌单
	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "空歌单",
	}
	db.PlaylistRepository().Create(ctx, playlist)

	// 获取歌单歌曲
	songs, err := db.PlaylistSongRepository().GetSongs(ctx, playlist.ID)
	if err != nil {
		t.Fatalf("GetPlaylistSongs() error = %v", err)
	}

	if len(songs) != 0 {
		t.Errorf("GetPlaylistSongs() for empty playlist count = %v, want %v", len(songs), 0)
	}
}

// TestAddDuplicateSongToPlaylist 测试添加重复歌曲到歌单
func TestAddDuplicateSongToPlaylist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 创建歌单和歌曲
	playlist := &models.Playlist{
		Type: models.PlaylistTypeNormal,
		Name: "测试歌单",
	}
	db.PlaylistRepository().Create(ctx, playlist)

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}
	db.SongRepository().BatchCreate(ctx, []*models.Song{song})

	// 第一次添加
	err := db.PlaylistSongRepository().AddSong(ctx, playlist.ID, song.ID, 1)
	if err != nil {
		t.Fatalf("AddSongToPlaylist() first time error = %v", err)
	}

	// 第二次添加相同歌曲（应该失败，因为有 UNIQUE 约束）
	err = db.PlaylistSongRepository().AddSong(ctx, playlist.ID, song.ID, 2)
	if err == nil {
		t.Error("AddSongToPlaylist() should fail when adding duplicate song")
	}
}

// TestUpsertRemoteSongDedup 验证 (plugin_entry_path, dedup_key) 去重语义：
// 同一身份歌曲多次导入应命中已有 ID 并更新可变字段；不同身份独立 INSERT；
// 空 dedup_key 时退化为直接 INSERT（不去重）。
func TestUpsertRemoteSongDedup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 第一次导入：建立 dedup 基线
	first := &models.Song{
		Type:            models.TypeRemote,
		Title:           "晴天",
		Artist:          "周杰伦",
		Album:           "叶惠美",
		CoverURL:        "https://example.com/cover-old.jpg",
		Duration:        269,
		PluginEntryPath: "lxmusic",
		SourceData:      `{"platform":"qq","quality":"128k","songInfo":{"songmid":"abc"}}`,
		DedupKey:        "qq:abc",
	}
	if err := db.SongRepository().UpsertRemote(ctx, first); err != nil {
		t.Fatalf("first UpsertRemoteSong error: %v", err)
	}
	if first.ID == 0 {
		t.Fatal("first upsert should assign ID")
	}
	firstID := first.ID

	// 第二次导入同 dedup_key：必须命中已有 ID，并更新 source_data / 可变元数据
	second := &models.Song{
		Type:            models.TypeRemote,
		Title:           "晴天 (Remastered)",
		Artist:          "Jay Chou",
		Album:           "叶惠美 2024",
		CoverURL:        "https://example.com/cover-new.jpg",
		Duration:        270,
		PluginEntryPath: "lxmusic",
		SourceData:      `{"platform":"qq","quality":"320k","songInfo":{"songmid":"abc"}}`, // quality 变了
		DedupKey:        "qq:abc",
	}
	if err := db.SongRepository().UpsertRemote(ctx, second); err != nil {
		t.Fatalf("second UpsertRemoteSong error: %v", err)
	}
	if second.ID != firstID {
		t.Errorf("dedup miss: want id=%d (reuse), got id=%d (new row)", firstID, second.ID)
	}

	got, err := db.SongRepository().GetByID(ctx, firstID)
	if err != nil {
		t.Fatalf("GetSongByID error: %v", err)
	}
	if got.Title != "晴天 (Remastered)" {
		t.Errorf("title not updated: got %q", got.Title)
	}
	if got.CoverURL != "https://example.com/cover-new.jpg" {
		t.Errorf("cover_url not updated: got %q", got.CoverURL)
	}
	if got.SourceData != second.SourceData {
		t.Errorf("source_data not updated: got %q", got.SourceData)
	}

	// 不同 dedup_key：必须新建一条
	other := &models.Song{
		Type:            models.TypeRemote,
		Title:           "稻香",
		Artist:          "周杰伦",
		PluginEntryPath: "lxmusic",
		SourceData:      `{"platform":"qq","quality":"128k","songInfo":{"songmid":"xyz"}}`,
		DedupKey:        "qq:xyz",
	}
	if err := db.SongRepository().UpsertRemote(ctx, other); err != nil {
		t.Fatalf("other UpsertRemoteSong error: %v", err)
	}
	if other.ID == firstID || other.ID == 0 {
		t.Errorf("different dedup_key should create new row, got id=%d (firstID=%d)", other.ID, firstID)
	}

	// 同 dedup_key 不同 plugin_entry_path：也应独立
	otherPlugin := &models.Song{
		Type:            models.TypeRemote,
		Title:           "晴天",
		Artist:          "周杰伦",
		PluginEntryPath: "other-plugin",
		SourceData:      `{"x":1}`,
		DedupKey:        "qq:abc",
	}
	if err := db.SongRepository().UpsertRemote(ctx, otherPlugin); err != nil {
		t.Fatalf("otherPlugin UpsertRemoteSong error: %v", err)
	}
	if otherPlugin.ID == firstID || otherPlugin.ID == 0 {
		t.Errorf("different plugin_entry_path should create new row, got id=%d", otherPlugin.ID)
	}

	// 空 dedup_key（纯外链/老插件）：不去重，每次 INSERT
	pureA := &models.Song{
		Type:  models.TypeRemote,
		Title: "外链歌曲 A",
		URL:   "https://example.com/a.mp3",
	}
	pureB := &models.Song{
		Type:  models.TypeRemote,
		Title: "外链歌曲 A",
		URL:   "https://example.com/a.mp3",
	}
	if err := db.SongRepository().UpsertRemote(ctx, pureA); err != nil {
		t.Fatalf("pureA UpsertRemoteSong error: %v", err)
	}
	if err := db.SongRepository().UpsertRemote(ctx, pureB); err != nil {
		t.Fatalf("pureB UpsertRemoteSong error: %v", err)
	}
	if pureA.ID == 0 || pureB.ID == 0 || pureA.ID == pureB.ID {
		t.Errorf("empty dedup_key should INSERT every time, got pureA=%d pureB=%d", pureA.ID, pureB.ID)
	}
}
