package services

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"songloft/internal/database"
	"songloft/internal/database/testutil"
	"songloft/internal/models"

	"github.com/hanxi/tag"
)

// copyTestMP3 把 pkg/tag/testdata 里的样本 MP3 复制到一个临时路径，返回新路径。
// UpdateLyrics 测试需要一个真实可写的 MP3，复制后才能放心改 USLT 而不污染 testdata。
func copyTestMP3(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "pkg", "tag", "testdata", "with_tags", "sample.id3v23.mp3")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read sample mp3: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "sample.mp3")
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write tmp mp3: %v", err)
	}
	return dst
}

// readLyricsFromFile 读取本地音频文件的内嵌歌词。失败直接 fatal。
func readLyricsFromFile(t *testing.T, path string) (lyrics, title, artist string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audio: %v", err)
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		t.Fatalf("read tag: %v", err)
	}
	return m.Lyrics(), m.Title(), m.Artist()
}

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

func (m *mockTestScanner) ScanFiles(ctx context.Context, _ func(count int)) ([]string, error) {
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
	calls     int
	lastMode  string
	returnErr error
}

func (s *stubPlaylistAutoCreator) AutoCreate(ctx context.Context, playlistMode string, excludeDirs []string, coverStoragePath string, musicPath string) (*models.AutoCreatePlaylistsResponse, error) {
	s.calls++
	s.lastMode = playlistMode
	if s.returnErr != nil {
		return nil, s.returnErr
	}
	return &models.AutoCreatePlaylistsResponse{}, nil
}

// TestRunAutoCreatePlaylistsReadsConfig 验证扫描完成后会按配置 playlistMode 调用 AutoCreate，并切到 creating_playlists 阶段。
func TestRunAutoCreatePlaylistsReadsConfig(t *testing.T) {
	repo := newTestSongRepo(t)
	mdb := testutil.OpenMemoryDB(t)
	configService := NewConfigService(mdb.ConfigRepository())
	if err := configService.Set("scan_playlist_mode", "bubble_up"); err != nil {
		t.Fatalf("set config: %v", err)
	}

	stub := &stubPlaylistAutoCreator{}
	service := NewSongService(repo, nil, nil, nil, configService, stub)

	service.runAutoCreatePlaylists(context.Background())

	if stub.calls != 1 {
		t.Fatalf("AutoCreate calls = %d, want 1", stub.calls)
	}
	if stub.lastMode != "bubble_up" {
		t.Errorf("AutoCreate playlistMode = %q, want bubble_up", stub.lastMode)
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

// TestIsUnderScope 验证作用域前缀判断（Issue #262 定向扫描核心）。
func TestIsUnderScope(t *testing.T) {
	sep := string(filepath.Separator)
	roots := []string{filepath.Join("/music", "A"), filepath.Join("/music", "B")}
	tests := []struct {
		name       string
		path       string
		scopeRoots []string
		want       bool
	}{
		{"empty scope means all", "/music/C/x.mp3", nil, true},
		{"under root A", filepath.Join("/music", "A", "x.mp3"), roots, true},
		{"root A itself", filepath.Join("/music", "A"), roots, true},
		{"under root B nested", filepath.Join("/music", "B", "sub", "y.mp3"), roots, true},
		{"outside scope", filepath.Join("/music", "C", "z.mp3"), roots, false},
		// 前缀陷阱：/music/AB 不应被 /music/A 命中。
		{"sibling prefix trap", "/music" + sep + "AB" + sep + "z.mp3", roots, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnderScope(tt.path, tt.scopeRoots); got != tt.want {
				t.Errorf("isUnderScope(%q, %v) = %v, want %v", tt.path, tt.scopeRoots, got, tt.want)
			}
		})
	}
}

// TestCleanStaleRecordsScoped 验证定向扫描时过期清理只作用于作用域内，作用域外记录保留。
func TestCleanStaleRecordsScoped(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// 两条本地记录，其 file_path 均指向磁盘上不存在的文件（os.Stat 会失败→视为过期候选）。
	root := t.TempDir()
	inScope := filepath.Join(root, "A", "gone.mp3")  // 作用域内
	outScope := filepath.Join(root, "B", "gone.mp3") // 作用域外
	for _, p := range []string{inScope, outScope} {
		s := &models.Song{Type: models.TypeLocal, Title: filepath.Base(p), FilePath: p}
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create song %s: %v", p, err)
		}
	}

	existing, err := repo.ListLocalPaths(ctx)
	if err != nil {
		t.Fatalf("ListLocalPaths: %v", err)
	}

	// 定向扫描 A：scannedFiles 为空（A 下没扫到文件），scopeRoots 只含 A。
	cleaned := service.cleanStaleRecords(ctx, nil, existing, []string{filepath.Join(root, "A")})
	if cleaned != 1 {
		t.Fatalf("scoped cleanStaleRecords cleaned = %d, want 1", cleaned)
	}

	// 作用域内记录应被删，作用域外记录应保留。
	remaining, _ := repo.ListLocalPaths(ctx)
	if _, ok := remaining[inScope]; ok {
		t.Error("in-scope stale record should be deleted")
	}
	if _, ok := remaining[outScope]; !ok {
		t.Error("out-of-scope record must be preserved by scoped scan")
	}

	// 全库清理（scopeRoots=nil）应把剩下的作用域外过期记录也清掉。
	cleaned2 := service.cleanStaleRecords(ctx, nil, remaining, nil)
	if cleaned2 != 1 {
		t.Fatalf("full cleanStaleRecords cleaned = %d, want 1", cleaned2)
	}
	final, _ := repo.ListLocalPaths(ctx)
	if len(final) != 0 {
		t.Errorf("after full clean, remaining = %d, want 0", len(final))
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

	if err := service.Delete(ctx, song.ID, false); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := service.GetByID(ctx, song.ID)
	if err == nil {
		t.Error("GetByID() should return error after deletion")
	}
}

// TestSongServiceDeleteSharedFilePath 验证多条本地歌曲行指向同一 file_path 时，
// deleteFiles=true 删除其中一条不会误删仍被另一条引用的物理文件；两条都删后文件才被删。
func TestSongServiceDeleteSharedFilePath(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// 真实临时文件，两条歌曲行共用同一 file_path。
	shared := filepath.Join(t.TempDir(), "shared.mp3")
	if err := os.WriteFile(shared, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write shared file: %v", err)
	}
	a := &models.Song{Type: models.TypeLocal, Title: "副本A", FilePath: shared}
	b := &models.Song{Type: models.TypeLocal, Title: "副本B", FilePath: shared}
	for _, s := range []*models.Song{a, b} {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create song %q: %v", s.Title, err)
		}
	}

	// 删 A（deleteFiles=true）：B 仍引用同一文件，文件必须保留。
	if err := service.Delete(ctx, a.ID, true); err != nil {
		t.Fatalf("Delete(A) error = %v", err)
	}
	if _, err := os.Stat(shared); err != nil {
		t.Fatalf("shared file should be kept while song B still references it, stat err = %v", err)
	}

	// 删 B（deleteFiles=true）：不再有引用，文件应被删除。
	if err := service.Delete(ctx, b.ID, true); err != nil {
		t.Fatalf("Delete(B) error = %v", err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Errorf("shared file should be deleted after last reference removed, stat err = %v", err)
	}
}

// TestSongServiceBatchDeleteSharedFilePath 验证 BatchDelete 场景下：同一 file_path 若同时被
// 删除集合外的歌曲引用则保留；集合内两条共用同一文件且外部无引用时删除。
func TestSongServiceBatchDeleteSharedFilePath(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// 场景1：keepFile 被删除集合外的 keeper 引用 → 删 target 不删文件。
	keepFile := filepath.Join(t.TempDir(), "keep.mp3")
	if err := os.WriteFile(keepFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write keep file: %v", err)
	}
	target := &models.Song{Type: models.TypeLocal, Title: "待删", FilePath: keepFile}
	keeper := &models.Song{Type: models.TypeLocal, Title: "保留", FilePath: keepFile}
	for _, s := range []*models.Song{target, keeper} {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if _, err := service.BatchDelete(ctx, []int64{target.ID}, true); err != nil {
		t.Fatalf("BatchDelete error = %v", err)
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Errorf("keepFile should be kept (still referenced by keeper), stat err = %v", err)
	}

	// 场景2：delFile 仅被删除集合内两条共用，外部无引用 → 文件删除。
	delFile := filepath.Join(t.TempDir(), "del.mp3")
	if err := os.WriteFile(delFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write del file: %v", err)
	}
	c := &models.Song{Type: models.TypeLocal, Title: "共用C", FilePath: delFile}
	d := &models.Song{Type: models.TypeLocal, Title: "共用D", FilePath: delFile}
	for _, s := range []*models.Song{c, d} {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	if _, err := service.BatchDelete(ctx, []int64{c.ID, d.ID}, true); err != nil {
		t.Fatalf("BatchDelete error = %v", err)
	}
	if _, err := os.Stat(delFile); !os.IsNotExist(err) {
		t.Errorf("delFile should be deleted (no remaining reference), stat err = %v", err)
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

// TestUpdateLyrics_LocalSong_WritesFile 验证 type=local 的歌曲 UpdateLyrics 后：
//  1. DB 的 lyric 列被更新成 LyricPayload JSON
//  2. 音频文件的 USLT 已被改写
//  3. 其它 ID3 字段保留文件里的原值
//  4. 返回 status=written
func TestUpdateLyrics_LocalSong_WritesFile(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	mp3Path := copyTestMP3(t)
	if err := tag.WriteTag(mp3Path, tag.WriteOptions{Title: "原标题", Artist: "原艺术家"}); err != nil {
		t.Fatal(err)
	}

	song := &models.Song{
		Type:        models.TypeLocal,
		Title:       "数据库标题",
		Artist:      "数据库艺术家",
		Album:       "原专辑",
		FilePath:    mp3Path,
		Lyric:       "",
		LyricSource: models.LyricSourceEmbedded,
	}
	if err := repo.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	newLyric := models.LyricPayload{Lyric: "[00:01.500]hello\n[00:03.000]world"}.MarshalString()

	status, err := service.UpdateLyrics(ctx, song.ID, newLyric, models.LyricSourceManual, "")
	if err != nil {
		t.Fatalf("UpdateLyrics() error = %v", err)
	}
	if status != FileWriteWritten {
		t.Errorf("UpdateLyrics() status = %v, want %v", status, FileWriteWritten)
	}

	// DB 校验
	got, err := repo.GetByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Lyric != newLyric {
		t.Errorf("DB lyric = %q, want %q", got.Lyric, newLyric)
	}
	if got.LyricSource != models.LyricSourceManual {
		t.Errorf("DB lyric_source = %q, want %q", got.LyricSource, models.LyricSourceManual)
	}

	// 文件校验：USLT 含新主歌词，Title/Artist 未被清空
	fileLyrics, fileTitle, fileArtist := readLyricsFromFile(t, mp3Path)
	if fileLyrics != "[00:01.500]hello\n[00:03.000]world" {
		t.Errorf("file lyrics = %q, want new lyric text", fileLyrics)
	}
	if fileTitle != "原标题" {
		t.Errorf("file title = %q, want %q (must not be wiped)", fileTitle, "原标题")
	}
	if fileArtist != "原艺术家" {
		t.Errorf("file artist = %q, want %q (must not be wiped)", fileArtist, "原艺术家")
	}
}

func TestUpdateLyricsSharedCueKeepsFile(t *testing.T) {
	db := testutil.OpenMemoryDB(t)
	repo := db.SongRepository()
	service := NewSongService(repo, db, nil, nil, nil, nil)
	ctx := context.Background()
	path := copyTestMP3(t)
	song := &models.Song{Type: models.TypeLocal, Title: "CUE track", FilePath: path, CueSourcePath: path + ".cue"}
	if err := repo.Create(ctx, song); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lyric := models.LyricPayload{Lyric: "[00:01]CUE track lyric"}.MarshalString()
	if status, err := service.UpdateLyrics(ctx, song.ID, lyric, models.LyricSourceManual, ""); err != nil || status != FileWriteUnchanged {
		t.Fatalf("shared file write: %s %v", status, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("shared file was modified: %v", err)
	}
	stored, err := repo.GetByID(ctx, song.ID)
	if err != nil || stored.Lyric != lyric {
		t.Fatalf("CUE lyric was not saved: %+v %v", stored, err)
	}
}

// TestUpdateLyrics_RemoteSong_SkipsFile 验证 type=remote 的歌曲不会回写文件，
// 只更新 DB，返回 status=skipped。
func TestUpdateLyrics_RemoteSong_SkipsFile(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	song := &models.Song{
		Type:  models.TypeRemote,
		Title: "远程歌曲",
		URL:   "https://example.com/song.mp3",
		Lyric: "",
	}
	if err := repo.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	status, err := service.UpdateLyrics(ctx, song.ID, `{"lyric":"[00:01]x"}`, models.LyricSourceCached, "")
	if err != nil {
		t.Fatalf("UpdateLyrics() error = %v", err)
	}
	if status != FileWriteUnchanged {
		t.Errorf("UpdateLyrics() status = %v, want %v", status, FileWriteUnchanged)
	}

	got, err := repo.GetByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Lyric != `{"lyric":"[00:01]x"}` {
		t.Errorf("DB lyric not updated")
	}
}

// TestUpdateLyrics_URLSource_SkipsFile 验证 lyric_source=url 时跳过文件回写，
// lyric_remote_url 列被更新。
func TestUpdateLyrics_URLSource_SkipsFile(t *testing.T) {
	repo := newTestSongRepo(t)
	service := NewSongService(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	mp3Path := copyTestMP3(t)
	originalLyrics, _, _ := readLyricsFromFile(t, mp3Path)

	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    "本地歌曲",
		FilePath: mp3Path,
	}
	if err := repo.Create(ctx, song); err != nil {
		t.Fatalf("create song: %v", err)
	}

	status, err := service.UpdateLyrics(ctx, song.ID, "", models.LyricSourceURL, "https://lrc.example.com/x")
	if err != nil {
		t.Fatalf("UpdateLyrics() error = %v", err)
	}
	if status != FileWriteUnchanged {
		t.Errorf("UpdateLyrics() status = %v, want %v", status, FileWriteUnchanged)
	}

	got, _ := repo.GetByID(ctx, song.ID)
	if got.LyricRemoteURL != "https://lrc.example.com/x" {
		t.Errorf("DB lyric_remote_url = %q", got.LyricRemoteURL)
	}

	// 文件 USLT 未被改动
	fileLyrics, _, _ := readLyricsFromFile(t, mp3Path)
	if fileLyrics != originalLyrics {
		t.Errorf("file USLT changed for url source: %q -> %q", originalLyrics, fileLyrics)
	}
}
