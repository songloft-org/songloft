package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

// newTestSongRepo 启动 :memory: SQLite，返回 SongRepository。
func newTestSongRepo(t *testing.T) *database.SongRepository {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	return mdb.SongRepository()
}

// TestSongServiceGetByID 测试获取歌曲
func TestSongServiceGetByID(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "测试歌曲",
		FilePath: "/music/test.mp3",
	}
	if err := repo.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	got, err := service.GetByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Title != song.Title {
		t.Errorf("GetByID() Title = %v, want %v", got.Title, song.Title)
	}

	_, err = service.GetByID(ctx, 99999)
	if err == nil {
		t.Error("GetByID() should return error for non-existent song")
	}
}

// TestSongServiceUpdate 测试更新歌曲
func TestSongServiceUpdate(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	song := &models.Song{
		Type: models.TypeLocal, Title: "原标题", FilePath: "/music/test.mp3",
	}
	if err := repo.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	song.Title = "新标题"
	if err := service.Update(ctx, song); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ := service.GetByID(ctx, song.ID)
	if got.Title != "新标题" {
		t.Errorf("Update() Title = %v, want %v", got.Title, "新标题")
	}
}

// mockTestScanner 实现 Scanner 接口用于测试
type mockTestScanner struct {
	files   []string
	scanErr error
}

func (m *mockTestScanner) ScanFiles(ctx context.Context) ([]string, error) {
	if m.scanErr != nil {
		return nil, m.scanErr
	}
	return m.files, nil
}

func (m *mockTestScanner) GetFileInfo(path string) (*FileInfo, error) {
	return &FileInfo{
		Path: path,
		Size: 1024000,
	}, nil
}

func (m *mockTestScanner) IsAudioFile(filename string) bool {
	return true
}

func (m *mockTestScanner) ShouldExcludeDir(dirPath string) bool {
	return false
}

// TestSongServiceScanAndImportNote 说明 ScanAndImport 测试策略
func TestSongServiceScanAndImportNote(t *testing.T) {
	t.Skip("ScanAndImport 方法依赖文件系统和 ffprobe，应在集成测试环境中测试")
}

// stubPlaylistAutoCreator 记录 AutoCreate 的调用参数，用于验证扫描完成后的串接逻辑。
type stubPlaylistAutoCreator struct {
	calls       int
	lastInclude bool
	returnErr   error
}

func (s *stubPlaylistAutoCreator) AutoCreate(ctx context.Context, includeSubdirs bool) (*models.AutoCreatePlaylistsResponse, error) {
	s.calls++
	s.lastInclude = includeSubdirs
	if s.returnErr != nil {
		return nil, s.returnErr
	}
	return &models.AutoCreatePlaylistsResponse{}, nil
}

// TestRunAutoCreatePlaylistsReadsConfig 验证扫描完成后会按配置 includeSubdirs 调用 AutoCreate，并切到 creating_playlists 阶段。
func TestRunAutoCreatePlaylistsReadsConfig(t *testing.T) {
	repo := newTestSongRepo(t)
	mdb := testutil.OpenMemoryDB(t)
	configService := NewConfigService(mdb.ConfigRepository())
	if err := configService.Set("scan_auto_create_include_subdirs", "true"); err != nil {
		t.Fatalf("set config: %v", err)
	}

	stub := &stubPlaylistAutoCreator{}
	service := NewSongService(repo, nil, nil, nil, configService, stub)

	service.runAutoCreatePlaylists(context.Background())

	if stub.calls != 1 {
		t.Fatalf("AutoCreate calls = %d, want 1", stub.calls)
	}
	if !stub.lastInclude {
		t.Errorf("AutoCreate includeSubdirs = false, want true")
	}
	if got := service.scanProgressManager.GetProgress().Status; got != ScanStatusCreatingPlaylists {
		t.Errorf("scan status = %v, want %v", got, ScanStatusCreatingPlaylists)
	}
}

// TestRunAutoCreatePlaylistsSkipsWhenNil 验证没注入 PlaylistAutoCreator 时不会 panic。
func TestRunAutoCreatePlaylistsSkipsWhenNil(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	service.runAutoCreatePlaylists(context.Background())
	if got := service.scanProgressManager.GetProgress().Status; got == ScanStatusCreatingPlaylists {
		t.Errorf("scan status should not be creating_playlists when stub absent")
	}
}

// TestSongServiceDelete 测试删除歌曲
func TestSongServiceDelete(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	song := &models.Song{
		Type: models.TypeLocal, Title: "测试歌曲", FilePath: "/music/test.mp3",
	}
	if err := repo.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	if err := service.Delete(ctx, song.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := service.GetByID(ctx, song.ID)
	if err == nil {
		t.Error("GetByID() should return error after deletion")
	}
}

// TestSongServiceList 测试列出歌曲
func TestSongServiceList(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	seed := []*models.Song{
		{Type: models.TypeLocal, Title: "本地歌曲1", FilePath: "/music/1.mp3"},
		{Type: models.TypeLocal, Title: "本地歌曲2", FilePath: "/music/2.mp3"},
		{Type: models.TypeRemote, Title: "网络歌曲", URL: "https://example.com/song.mp3"},
	}
	for _, s := range seed {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create song: %v", err)
		}
	}

	filter := &database.SongFilter{}
	list, err := service.List(ctx, filter)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 3 {
		t.Errorf("List() count = %v, want %v", len(list), 3)
	}

	filter = &database.SongFilter{Type: models.TypeLocal}
	list, err = service.List(ctx, filter)
	if err != nil {
		t.Fatalf("List() with filter error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List() with filter count = %v, want %v", len(list), 2)
	}
}

// TestSongServiceSearch 测试搜索歌曲
func TestSongServiceSearch(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	if err := repo.Create(ctx, &models.Song{
		Type: models.TypeLocal, Title: "测试歌曲", FilePath: "/music/test.mp3",
	}); err != nil {
		t.Fatalf("create song: %v", err)
	}

	songs, err := service.Search(ctx, "测试", "", 10, 0)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(songs) == 0 {
		t.Error("Search() should return results")
	}
}

// TestSongServiceCount 测试统计歌曲
func TestSongServiceCount(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	seed := []*models.Song{
		{Type: models.TypeLocal, Title: "歌曲1", FilePath: "/music/1.mp3"},
		{Type: models.TypeLocal, Title: "歌曲2", FilePath: "/music/2.mp3"},
		{Type: models.TypeRemote, Title: "歌曲3", URL: "https://example.com/3.mp3"},
	}
	for _, s := range seed {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create song: %v", err)
		}
	}

	filter := &database.SongFilter{}
	count, err := service.Count(ctx, filter)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 3 {
		t.Errorf("Count() = %v, want %v", count, 3)
	}

	filter = &database.SongFilter{Type: models.TypeLocal}
	count, err = service.Count(ctx, filter)
	if err != nil {
		t.Fatalf("Count() with filter error = %v", err)
	}
	if count != 2 {
		t.Errorf("Count() with filter = %v, want %v", count, 2)
	}
}

// TestSongServiceAddRemoteSongs 测试批量添加网络歌曲
func TestSongServiceAddRemoteSongs(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	inputs := []RemoteSongInput{
		{URL: "https://example.com/song1.mp3", Title: "网络歌曲1", Artist: "艺术家", Album: "专辑", CoverURL: "https://example.com/cover.jpg", Duration: 253.5},
		{URL: "https://example.com/song2.mp3", Title: "网络歌曲2", Duration: 120.0},
	}

	songs, err := service.AddRemoteSongs(ctx, inputs)
	if err != nil {
		t.Fatalf("AddRemoteSongs() error = %v", err)
	}

	if len(songs) != 2 {
		t.Fatalf("AddRemoteSongs() returned %d songs, want 2", len(songs))
	}
	if songs[0].ID == 0 {
		t.Error("AddRemoteSongs() should set song ID")
	}
	if songs[0].Type != models.TypeRemote {
		t.Errorf("AddRemoteSongs() Type = %v, want %v", songs[0].Type, models.TypeRemote)
	}
	if songs[0].Title != "网络歌曲1" {
		t.Errorf("AddRemoteSongs() Title = %v, want 网络歌曲1", songs[0].Title)
	}
	if songs[0].Duration != 253.5 {
		t.Errorf("AddRemoteSongs() Duration = %v, want 253.5", songs[0].Duration)
	}
}

// TestSongServiceAddRadios 测试批量添加电台/广播
func TestSongServiceAddRadios(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	inputs := []RadioInput{
		{URL: "https://example.com/radio1.m3u8", Title: "测试电台1", CoverURL: "https://example.com/cover.jpg"},
		{URL: "https://example.com/radio2.m3u8", Title: "测试电台2"},
	}

	songs, err := service.AddRadios(ctx, inputs)
	if err != nil {
		t.Fatalf("AddRadios() error = %v", err)
	}

	if len(songs) != 2 {
		t.Fatalf("AddRadios() returned %d songs, want 2", len(songs))
	}
	if songs[0].ID == 0 {
		t.Error("AddRadios() should set song ID")
	}
	if songs[0].Type != models.TypeRadio {
		t.Errorf("AddRadios() Type = %v, want %v", songs[0].Type, models.TypeRadio)
	}
	if !songs[0].IsLive {
		t.Error("AddRadios() should set IsLive to true")
	}
}

// TestCleanInvalidSongsWithExcludedDirs 测试清理排除目录中的歌曲
func TestCleanInvalidSongsWithExcludedDirs(t *testing.T) {
	repo := newTestSongRepo(t)
	ctx := context.Background()

	tmpDir := t.TempDir()

	existingFiles := []string{
		"normal/song1.mp3",
		"excluded_path/song2.mp3",
		"deep/@eaDir/song3.mp3",
		"good/song4.mp3",
	}
	for _, f := range existingFiles {
		fullPath := filepath.Join(tmpDir, f)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	scanner := NewScanner(&ScanConfig{
		MusicPath:    tmpDir,
		ExcludeDirs:  []string{"@eaDir"},
		ExcludePaths: []string{filepath.Join(tmpDir, "excluded_path")},
	})

	service := NewSongService(repo, nil, nil, scanner, nil, nil)

	seed := []*models.Song{
		{Type: models.TypeLocal, Title: "正常歌曲", FilePath: filepath.Join(tmpDir, "normal/song1.mp3")},
		{Type: models.TypeLocal, Title: "排除路径中的歌曲", FilePath: filepath.Join(tmpDir, "excluded_path/song2.mp3")},
		{Type: models.TypeLocal, Title: "排除名称中的歌曲", FilePath: filepath.Join(tmpDir, "deep/@eaDir/song3.mp3")},
		{Type: models.TypeLocal, Title: "不存在的歌曲", FilePath: filepath.Join(tmpDir, "nonexistent/song5.mp3")},
		{Type: models.TypeLocal, Title: "正常歌曲2", FilePath: filepath.Join(tmpDir, "good/song4.mp3")},
	}
	for _, s := range seed {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create song: %v", err)
		}
	}
	// 索引：title -> id，方便后续校验
	idByTitle := make(map[string]int64, len(seed))
	for _, s := range seed {
		idByTitle[s.Title] = s.ID
	}

	result, err := service.CleanInvalidSongs(ctx)
	if err != nil {
		t.Fatalf("CleanInvalidSongs() error = %v", err)
	}

	if result.Total != 3 {
		t.Errorf("CleanInvalidSongs() Total = %d, want 3", result.Total)
	}
	if result.FileNotFound != 1 {
		t.Errorf("CleanInvalidSongs() FileNotFound = %d, want 1", result.FileNotFound)
	}
	if result.InExcludedDir != 2 {
		t.Errorf("CleanInvalidSongs() InExcludedDir = %d, want 2", result.InExcludedDir)
	}

	// 校验保留：正常文件应仍可查
	if _, err := repo.GetByID(ctx, idByTitle["正常歌曲"]); err != nil {
		t.Errorf("Normal song should not be cleaned: %v", err)
	}
	if _, err := repo.GetByID(ctx, idByTitle["正常歌曲2"]); err != nil {
		t.Errorf("Normal song 2 should not be cleaned: %v", err)
	}

	// 校验清理：被排除/不存在的文件应已删除
	if _, err := repo.GetByID(ctx, idByTitle["排除路径中的歌曲"]); err == nil {
		t.Error("Song in excluded path should be cleaned")
	}
	if _, err := repo.GetByID(ctx, idByTitle["排除名称中的歌曲"]); err == nil {
		t.Error("Song in excluded dir name should be cleaned")
	}
	if _, err := repo.GetByID(ctx, idByTitle["不存在的歌曲"]); err == nil {
		t.Error("Non-existent song should be cleaned")
	}
}

// TestCleanInvalidSongsWithoutScanner 测试没有 Scanner 时的清理（仅清理不存在的文件）
func TestCleanInvalidSongsWithoutScanner(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	tmpDir := t.TempDir()

	existingFile := filepath.Join(tmpDir, "existing.mp3")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	seed := []*models.Song{
		{Type: models.TypeLocal, Title: "存在的歌曲", FilePath: existingFile},
		{Type: models.TypeLocal, Title: "不存在的歌曲", FilePath: filepath.Join(tmpDir, "nonexistent.mp3")},
	}
	for _, s := range seed {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create song: %v", err)
		}
	}

	result, err := service.CleanInvalidSongs(ctx)
	if err != nil {
		t.Fatalf("CleanInvalidSongs() error = %v", err)
	}

	if result.Total != 1 {
		t.Errorf("CleanInvalidSongs() Total = %d, want 1", result.Total)
	}
	if result.FileNotFound != 1 {
		t.Errorf("CleanInvalidSongs() FileNotFound = %d, want 1", result.FileNotFound)
	}
	if result.InExcludedDir != 0 {
		t.Errorf("CleanInvalidSongs() InExcludedDir = %d, want 0", result.InExcludedDir)
	}
}
