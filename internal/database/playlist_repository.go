package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"songloft/internal/database/sqlc"
	"songloft/internal/fileutil"
	"songloft/internal/models"
)

// PlaylistRepository 歌单仓储。
// 固定 SQL 走 sqlc.Queries；动态过滤的 List/Count/BatchDelete 走 squirrel；
// 复杂多语句操作（Create/Update/AutoCreate）在底层为 *sql.DB 时自启动事务，
// 在已有事务的连接上则不再嵌套事务。
type PlaylistRepository struct {
	db      sqlc.DBTX
	queries *sqlc.Queries
}

// NewPlaylistRepository 用 *sql.DB 或 *sql.Tx 构造仓储。
func NewPlaylistRepository(db sqlc.DBTX) *PlaylistRepository {
	return &PlaylistRepository{db: db, queries: sqlc.New(db)}
}

// Create 创建歌单。同名（不区分类型）已存在时返回 models.ErrPlaylistNameConflict。
// SQLite 是单 writer，事务内 SELECT + INSERT 即可保证并发场景下不会出现两条同名记录。
func (r *PlaylistRepository) Create(ctx context.Context, playlist *models.Playlist) error {
	labelsJSON, err := json.Marshal(playlist.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	return r.runInTx(ctx, func(dbtx sqlc.DBTX, q *sqlc.Queries) error {
		for _, label := range playlist.Labels {
			if !strings.HasPrefix(label, "tag:") {
				continue
			}
			id, ok := models.ParsePlaylistTagLabel(label)
			if !ok {
				return fmt.Errorf("invalid playlist tag label: %s", label)
			}
			if _, err := q.GetSongTagByID(ctx, id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
		}
		if _, err := q.FindPlaylistByName(ctx, playlist.Name); err == nil {
			return models.ErrPlaylistNameConflict
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check existing playlist: %w", err)
		}

		maxPos, err := q.GetMaxPlaylistPosition(ctx)
		if err != nil {
			return fmt.Errorf("get max position: %w", err)
		}

		id, err := q.CreatePlaylist(ctx, sqlc.CreatePlaylistParams{
			Type:        playlist.Type,
			Name:        playlist.Name,
			Description: playlist.Description,
			CoverPath:   playlist.CoverPath,
			CoverUrl:    playlist.CoverURL,
			Labels:      string(labelsJSON),
			Position:    maxPos + 1,
		})
		if err != nil {
			return fmt.Errorf("insert playlist: %w", err)
		}

		now := time.Now()
		playlist.ID = id
		playlist.CreatedAt = now
		playlist.UpdatedAt = now
		return nil
	})
}

// GetByID 根据 ID 获取歌单，找不到返回 ErrNotFound。
func (r *PlaylistRepository) GetByID(ctx context.Context, id int64) (*models.Playlist, error) {
	row, err := r.queries.GetPlaylistByID(ctx, sqlc.GetPlaylistByIDParams{
		PlaylistID: id,
		ID:         id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get playlist %d: %w", id, err)
	}
	return playlistRowToModel(row), nil
}

// FindByName 按名称精确查找歌单，找不到返回 ErrNotFound。
func (r *PlaylistRepository) FindByName(ctx context.Context, name string) (*models.Playlist, error) {
	id, err := r.queries.FindPlaylistByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find playlist by name %q: %w", name, err)
	}
	return r.GetByID(ctx, id)
}

// Update 更新歌单。改名冲突返回 models.ErrPlaylistNameConflict，找不到返回 ErrNotFound。
func (r *PlaylistRepository) Update(ctx context.Context, playlist *models.Playlist) error {
	now := time.Now()
	return r.runInTx(ctx, func(dbtx sqlc.DBTX, q *sqlc.Queries) error {
		// Custom tag relationships are written through the tag API, never through
		// a stale metadata object that could restore a deleted/reused tag ID.
		existing, err := q.GetPlaylistByID(ctx, sqlc.GetPlaylistByIDParams{PlaylistID: playlist.ID, ID: playlist.ID})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		labels := make([]string, 0, len(playlist.Labels))
		for _, label := range playlist.Labels {
			if !strings.HasPrefix(label, "tag:") {
				labels = append(labels, label)
			}
		}
		for _, label := range parseLabelsJSON(sql.NullString{String: existing.Labels, Valid: true}) {
			if strings.HasPrefix(label, "tag:") {
				labels = append(labels, label)
			}
		}
		labelsJSON, err := json.Marshal(labels)
		if err != nil {
			return err
		}
		if _, err := q.FindPlaylistByNameExcludeID(ctx, sqlc.FindPlaylistByNameExcludeIDParams{
			Name: playlist.Name,
			ID:   playlist.ID,
		}); err == nil {
			return models.ErrPlaylistNameConflict
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check existing playlist: %w", err)
		}

		rows, err := q.UpdatePlaylist(ctx, sqlc.UpdatePlaylistParams{
			Name:        playlist.Name,
			Description: playlist.Description,
			CoverPath:   playlist.CoverPath,
			CoverUrl:    playlist.CoverURL,
			Labels:      string(labelsJSON),
			UpdatedAt:   now,
			ID:          playlist.ID,
		})
		if err != nil {
			return fmt.Errorf("update playlist %d: %w", playlist.ID, err)
		}
		if rows == 0 {
			return ErrNotFound
		}
		playlist.UpdatedAt = now
		playlist.Labels = labels
		return nil
	})
}

// UpdateSort 更新歌单的视图排序偏好，找不到返回 ErrNotFound。
func (r *PlaylistRepository) UpdateSort(ctx context.Context, id int64, sortBy, sortOrder string) error {
	rows, err := r.queries.UpdatePlaylistSort(ctx, sqlc.UpdatePlaylistSortParams{
		SortBy:    sortBy,
		SortOrder: sortOrder,
		ID:        id,
	})
	if err != nil {
		return fmt.Errorf("update playlist sort %d: %w", id, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Touch 更新歌单的 updated_at 时间戳，找不到返回 ErrNotFound。
func (r *PlaylistRepository) Touch(ctx context.Context, id int64) error {
	rows, err := r.queries.TouchPlaylist(ctx, sqlc.TouchPlaylistParams{
		UpdatedAt: time.Now(),
		ID:        id,
	})
	if err != nil {
		return fmt.Errorf("touch playlist %d: %w", id, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete 删除歌单，找不到返回 ErrNotFound。playlist_songs 由 FK ON DELETE CASCADE 自动清理。
func (r *PlaylistRepository) Delete(ctx context.Context, id int64) error {
	rows, err := r.queries.DeletePlaylist(ctx, id)
	if err != nil {
		return fmt.Errorf("delete playlist %d: %w", id, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// List 按过滤条件 + 白名单排序 + 分页拉取歌单。
func (r *PlaylistRepository) List(ctx context.Context, filter *PlaylistFilter) ([]*models.Playlist, error) {
	if filter == nil {
		filter = &PlaylistFilter{}
	}
	sb := playlistSelectBuilder()
	sb = applyPlaylistFilter(sb, filter, "p.")
	// 置顶优先级：置顶歌单永远排最前，同为置顶时按置顶时间倒序（最近置顶在前）；
	// squirrel 的 OrderBy 是累加而非覆盖，故这里先加置顶排序，再叠加下面的原有排序作为 tiebreaker。
	sb = sb.OrderBy("p.pinned_at IS NULL ASC", "p.pinned_at DESC")
	sb = applyOrder(sb, filter.OrderBy, filter.Order, "p.position ASC, p.updated_at DESC", playlistOrderWhitelist, "p.")
	sb = applyPagination(sb, filter.Limit, filter.Offset)

	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list playlists sql: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}
	defer rows.Close()

	playlists := []*models.Playlist{}
	for rows.Next() {
		p, err := scanPlaylistRow(rows)
		if err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate playlists: %w", err)
	}
	return playlists, nil
}

// Count 与 List 共享过滤条件，返回匹配行数。
func (r *PlaylistRepository) Count(ctx context.Context, filter *PlaylistFilter) (int64, error) {
	if filter == nil {
		filter = &PlaylistFilter{}
	}
	sb := sq.Select("COUNT(*)").From("playlists")
	sb = applyPlaylistFilter(sb, filter, "")

	query, args, err := sb.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count playlists sql: %w", err)
	}
	var n int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count playlists: %w", err)
	}
	return n, nil
}

// BatchDelete 批量删除歌单，跳过带 built_in 标签的内置歌单，返回实际删除条数。
func (r *PlaylistRepository) BatchDelete(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// squirrel 没有 json_each 抽象，这里手工拼接 IN 列表 + NOT EXISTS。
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, models.PlaylistLabelBuiltIn)
	query := fmt.Sprintf(`DELETE FROM playlists
		WHERE id IN (%s)
		AND NOT EXISTS (SELECT 1 FROM json_each(labels) WHERE value = ?)`,
		strings.Join(placeholders, ","))

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("batch delete playlists: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return int(affected), nil
}

// DeleteAutoCreated 删除所有带 auto_created 标签的歌单（playlist_songs 由 FK CASCADE 清理）。
func (r *PlaylistRepository) DeleteAutoCreated(ctx context.Context) error {
	query := `DELETE FROM playlists
		WHERE EXISTS (SELECT 1 FROM json_each(labels) WHERE value = ?)`
	if _, err := r.db.ExecContext(ctx, query, models.PlaylistLabelAutoCreated); err != nil {
		return fmt.Errorf("delete auto-created playlists: %w", err)
	}
	return nil
}

// SetPinned 设置歌单的置顶时间，pinnedAt 为 nil 表示取消置顶，找不到返回 ErrNotFound。
func (r *PlaylistRepository) SetPinned(ctx context.Context, id int64, pinnedAt *time.Time) error {
	var v sql.NullTime
	if pinnedAt != nil {
		v = sql.NullTime{Time: *pinnedAt, Valid: true}
	}
	rows, err := r.queries.SetPlaylistPinned(ctx, sqlc.SetPlaylistPinnedParams{PinnedAt: v, ID: id})
	if err != nil {
		return fmt.Errorf("set playlist pinned %d: %w", id, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// BatchUpdatePositions 按给定 ID 顺序更新歌单 position（1..N）。
func (r *PlaylistRepository) BatchUpdatePositions(ctx context.Context, playlistIDs []int64) error {
	return r.runInTx(ctx, func(dbtx sqlc.DBTX, q *sqlc.Queries) error {
		for i, id := range playlistIDs {
			rows, err := q.UpdatePlaylistPosition(ctx, sqlc.UpdatePlaylistPositionParams{
				Position: int64(i + 1),
				ID:       id,
			})
			if err != nil {
				return fmt.Errorf("update position for playlist %d: %w", id, err)
			}
			if rows == 0 {
				return fmt.Errorf("playlist %d not found", id)
			}
		}
		return nil
	})
}

// AutoCreate 根据本地歌曲的目录结构批量生成歌单，并把每首歌写入对应歌单。
// 写操作集中在单一事务里：清理旧的 auto_created 歌单 → 插入新歌单 → 批量插入 playlist_songs。
// playlistMode: "directory"（按文件夹）、"top_level"（按顶层文件夹合并）、"bubble_up"（向上冒泡）。
// excludeDirs 指定在自动创建歌单时要排除的目录名称（按名称匹配，路径中任何层级包含该名称都会被排除）。
func (r *PlaylistRepository) AutoCreate(ctx context.Context, playlistMode string, excludeDirs []string, coverStoragePath string, musicPath string) (*models.AutoCreatePlaylistsResponse, error) {
	songRepo := NewSongRepository(r.db)
	songs, err := songRepo.List(ctx, &SongFilter{
		Type:  models.TypeLocal,
		Limit: models.MaxPaginationLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list songs: %w", err)
	}

	if len(songs) == 0 {
		if err := r.DeleteAutoCreated(ctx); err != nil {
			return nil, err
		}
		return &models.AutoCreatePlaylistsResponse{
			Playlists: []models.PlaylistInfo{},
			Total:     0,
		}, nil
	}

	// 构建排除目录集合，用于快速查找
	excludeSet := make(map[string]struct{}, len(excludeDirs))
	for _, d := range excludeDirs {
		excludeSet[d] = struct{}{}
	}

	// 检查路径是否包含排除目录
	shouldExcludeDir := func(dir string) bool {
		for _, part := range strings.Split(dir, "/") {
			if _, ok := excludeSet[part]; ok {
				return true
			}
		}
		return false
	}

	// 分离 CUE 歌曲和普通歌曲
	type cueAlbumInfo struct {
		name    string // 专辑名
		desc    string // 描述
		songIDs []int64
	}
	cueAlbums := make(map[string]*cueAlbumInfo) // key: cue_source_path

	// 收集所有 CUE 轨的音频源文件路径：这些文件作为整轨镜像已由 CUE 专辑歌单覆盖，
	// 不应再作为独立歌曲出现在目录歌单中（否则同一内容会同时出现在两个歌单里）。
	cueAudioFiles := make(map[string]struct{})
	for _, song := range songs {
		if song.CueSourcePath != "" && song.FilePath != "" {
			cueAudioFiles[song.FilePath] = struct{}{}
		}
	}

	// 按「目录名/文件名」去重：同一物理文件若因路径格式不同（相对/绝对）
	// 存在多条 song 行，只保留第一条参与歌单分组，避免产生重复歌单。
	type pathKey struct{ dir, file string }
	seenFiles := make(map[pathKey]struct{}, len(songs))
	dedupSongs := make([]*models.Song, 0, len(songs))
	for _, song := range songs {
		if song.CueSourcePath != "" || song.FilePath == "" {
			dedupSongs = append(dedupSongs, song)
			continue
		}
		key := pathKey{
			dir:  filepath.Base(filepath.Dir(song.FilePath)),
			file: filepath.Base(song.FilePath),
		}
		if _, dup := seenFiles[key]; dup {
			continue
		}
		seenFiles[key] = struct{}{}
		dedupSongs = append(dedupSongs, song)
	}
	songs = dedupSongs

	// TopLevel 模式需要公共路径前缀来提取相对首层目录；
	// 绝对路径直接 SplitN("/",2) 在 Linux 上得 ""、Windows 上得 "C:"，全部歌归到一个歌单。
	var musicRoot string
	if playlistMode == models.PlaylistModeTopLevel {
		var songDirs []string
		for _, song := range songs {
			if song.FilePath != "" && song.CueSourcePath == "" {
				d := filepath.ToSlash(filepath.Dir(song.FilePath))
				if !shouldExcludeDir(d) {
					songDirs = append(songDirs, d)
				}
			}
		}
		musicRoot = findCommonPathPrefix(songDirs)
	}

	// BubbleUp 模式需要 music_path 作为向上冒泡的停止边界（#428）。
	var bubbleRoot string
	if musicPath != "" {
		bubbleRoot = filepath.ToSlash(filepath.Clean(musicPath))
	}

	dirToSongs := make(map[string][]int64)
	for _, song := range songs {
		if song.FilePath == "" {
			continue
		}

		// CUE 歌曲按 cue_source_path 分组
		if song.CueSourcePath != "" {
			album, ok := cueAlbums[song.CueSourcePath]
			if !ok {
				albumName := song.Album
				if albumName == "" {
					albumName = strings.TrimSuffix(filepath.Base(song.CueSourcePath), filepath.Ext(song.CueSourcePath))
				}
				album = &cueAlbumInfo{
					name: albumName,
					desc: filepath.Base(song.CueSourcePath),
				}
				cueAlbums[song.CueSourcePath] = album
			}
			album.songIDs = append(album.songIDs, song.ID)
			continue
		}

		// 跳过已有 CUE 轨的整轨音频文件
		if _, isCueSource := cueAudioFiles[song.FilePath]; isCueSource {
			continue
		}

		dir := filepath.ToSlash(filepath.Dir(song.FilePath))
		if shouldExcludeDir(dir) {
			continue
		}

		switch playlistMode {
		case models.PlaylistModeTopLevel:
			rel := strings.TrimPrefix(dir, musicRoot)
			rel = strings.TrimPrefix(rel, "/")
			if rel == "" {
				dirToSongs[dir] = append(dirToSongs[dir], song.ID)
			} else {
				topDir := musicRoot + "/" + strings.SplitN(rel, "/", 2)[0]
				dirToSongs[topDir] = append(dirToSongs[topDir], song.ID)
			}
		case models.PlaylistModeBubbleUp:
			dirToSongs[dir] = append(dirToSongs[dir], song.ID)
			parent := filepath.ToSlash(filepath.Dir(dir))
			for parent != "." && parent != "/" && parent != dir {
				if bubbleRoot != "" && len(parent) < len(bubbleRoot) {
					break
				}
				if !shouldExcludeDir(parent) {
					dirToSongs[parent] = append(dirToSongs[parent], song.ID)
				}
				if parent == bubbleRoot {
					break
				}
				next := filepath.ToSlash(filepath.Dir(parent))
				if next == parent {
					break
				}
				parent = next
			}
		default:
			dirToSongs[dir] = append(dirToSongs[dir], song.ID)
		}
	}

	songIDToSong := make(map[int64]*models.Song, len(songs))
	for _, song := range songs {
		songIDToSong[song.ID] = song
	}

	dirs := make([]string, 0, len(dirToSongs))
	for dir := range dirToSongs {
		dirs = append(dirs, dir)
	}
	// 排序保证处理顺序确定：新歌单 ID 分配、以及重名时 resolveAutoCreatedName 的
	// 后缀归属都不随 map 迭代顺序漂移（复用 ID 的正确性依赖跨扫描的稳定顺序）。
	sort.Strings(dirs)
	nameMap, descMap := generateSmartPlaylistNames(dirs)

	labelsJSON, err := json.Marshal([]string{models.PlaylistLabelAutoCreated})
	if err != nil {
		return nil, fmt.Errorf("marshal labels: %w", err)
	}
	labelsStr := string(labelsJSON)

	response := &models.AutoCreatePlaylistsResponse{
		Playlists: make([]models.PlaylistInfo, 0, len(dirToSongs)),
	}

	type playlistSongEntry struct {
		playlistID int64
		songID     int64
		position   int
	}
	allPlaylistSongs := make([]playlistSongEntry, 0)

	err = r.runInTx(ctx, func(dbtx sqlc.DBTX, q *sqlc.Queries) error {
		// 加载已有的 auto_created 歌单（name -> id），用于按名字复用歌单 ID，
		// 避免每次扫描都 DELETE+重建导致 ID 变化（外部消费者如 miot 插件会缓存 ID）。
		existingAutoList, err := q.ListAutoCreatedPlaylists(ctx)
		if err != nil {
			return fmt.Errorf("load existing auto-created playlists: %w", err)
		}
		// 记录已有自动歌单的 ID 及其当前封面：重建时保留已有非空封面
		// （含用户手动上传到自动歌单的封面），避免每次扫描覆盖导致封面跳变。
		type existingAuto struct {
			id        int64
			coverPath string
			coverURL  string
		}
		existingAutoMap := make(map[string]existingAuto, len(existingAutoList))
		for _, p := range existingAutoList {
			existingAutoMap[p.Name] = existingAuto{id: p.ID, coverPath: p.CoverPath, coverURL: p.CoverUrl}
		}

		// 收集非 auto_created 歌单的名字（用户手动建/内置歌单），用于消歧。
		// 同名约束在 Create 里强制；auto-create 走直接 INSERT 绕过了检查，必须自己消歧。
		// 注意：不把旧 auto_created 名字放进来——那些名字要么被本次复用（匹配），
		// 要么被最后清理删除，不应参与新歌单的消歧。
		allNamesList, err := q.ListAllPlaylistNames(ctx)
		if err != nil {
			return fmt.Errorf("load existing playlist names: %w", err)
		}
		existingNames := make(map[string]struct{}, len(allNamesList))
		for _, name := range allNamesList {
			if _, isAuto := existingAutoMap[name]; isAuto {
				continue
			}
			existingNames[name] = struct{}{}
		}

		// upsertPlaylist 复用或新建一个 auto_created 歌单：
		// 若解析出的 name 命中已有 auto 歌单则 UPDATE 元数据 + 清空旧关联歌曲（保留 ID），
		// 否则 INSERT 新歌单。两种情况都收集 playlist_songs 并写入 response。
		upsertPlaylist := func(candidate, desc, coverPath, coverURL string, songIDs []int64) error {
			name := resolveAutoCreatedName(candidate, existingNames)
			existingNames[name] = struct{}{}

			var playlistID int64
			if existing, ok := existingAutoMap[name]; ok {
				// 保留已有非空封面（含用户手动上传的），仅在原封面为空时才用新挑的。
				writeCoverPath, writeCoverURL := coverPath, coverURL
				if existing.coverPath != "" || existing.coverURL != "" {
					writeCoverPath, writeCoverURL = existing.coverPath, existing.coverURL
				}
				if _, err := q.UpdateAutoCreatedPlaylistMeta(ctx, sqlc.UpdateAutoCreatedPlaylistMetaParams{
					Name:        name,
					Description: desc,
					CoverPath:   writeCoverPath,
					CoverUrl:    writeCoverURL,
					ID:          existing.id,
				}); err != nil {
					return fmt.Errorf("update auto-created playlist %s: %w", name, err)
				}
				if err := q.DeletePlaylistSongsByPlaylistID(ctx, existing.id); err != nil {
					return fmt.Errorf("clear playlist songs for %d: %w", existing.id, err)
				}
				delete(existingAutoMap, name) // 标记已匹配，避免最后被误删
				playlistID = existing.id
			} else {
				newID, err := q.InsertAutoCreatedPlaylist(ctx, sqlc.InsertAutoCreatedPlaylistParams{
					Type:        models.PlaylistTypeNormal,
					Name:        name,
					Description: desc,
					CoverPath:   coverPath,
					CoverUrl:    coverURL,
					Labels:      labelsStr,
				})
				if err != nil {
					return fmt.Errorf("create playlist %s: %w", name, err)
				}
				playlistID = newID
			}

			for i, songID := range songIDs {
				allPlaylistSongs = append(allPlaylistSongs, playlistSongEntry{
					playlistID: playlistID,
					songID:     songID,
					position:   i + 1,
				})
			}
			response.Playlists = append(response.Playlists, models.PlaylistInfo{
				PlaylistID: playlistID,
				Name:       name,
				SongCount:  len(songIDs),
			})
			return nil
		}

		// CUE 专辑歌单：按 cue_source_path 排序保证处理顺序确定（重名专辑的
		// 后缀归属不随 map 迭代顺序漂移），组内歌曲按 cue_track_index 排序。
		cuePaths := make([]string, 0, len(cueAlbums))
		for cuePath := range cueAlbums {
			cuePaths = append(cuePaths, cuePath)
		}
		sort.Strings(cuePaths)
		for _, cuePath := range cuePaths {
			album := cueAlbums[cuePath]
			sort.SliceStable(album.songIDs, func(i, j int) bool {
				si, sj := songIDToSong[album.songIDs[i]], songIDToSong[album.songIDs[j]]
				return si.CueTrackIndex < sj.CueTrackIndex
			})
			cueDir := filepath.Dir(cuePath)
			coverPath, coverURL := pickDirCover(cueDir, album.songIDs, songIDToSong, coverStoragePath)
			if err := upsertPlaylist(album.name, album.desc, coverPath, coverURL, album.songIDs); err != nil {
				return err
			}
		}

		// 目录歌单：按已排序的 dirs 顺序处理（确定性）
		for _, dir := range dirs {
			songIDs := dirToSongs[dir]
			// 按数字前缀排序歌曲，与 Flutter 端展示顺序一致。
			sort.SliceStable(songIDs, func(i, j int) bool {
				return lessSongByNumberThenTitle(songIDToSong[songIDs[i]], songIDToSong[songIDs[j]])
			})
			coverPath, coverURL := pickDirCover(dir, songIDs, songIDToSong, coverStoragePath)
			if err := upsertPlaylist(nameMap[dir], descMap[dir], coverPath, coverURL, songIDs); err != nil {
				return err
			}
		}

		// 清理未匹配的旧 auto_created 歌单（对应目录/专辑已不存在）。
		// CASCADE 自动删除其 playlist_songs。
		for _, stale := range existingAutoMap {
			if _, err := dbtx.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, stale.id); err != nil {
				return fmt.Errorf("delete stale auto-created playlist %d: %w", stale.id, err)
			}
		}

		// 多行 INSERT，每批最多 500 行，避免单条语句过长。
		const batchSize = 500
		for i := 0; i < len(allPlaylistSongs); i += batchSize {
			end := i + batchSize
			if end > len(allPlaylistSongs) {
				end = len(allPlaylistSongs)
			}
			batch := allPlaylistSongs[i:end]

			valueStrings := make([]string, 0, len(batch))
			valueArgs := make([]any, 0, len(batch)*3)
			for _, entry := range batch {
				valueStrings = append(valueStrings, "(?, ?, ?)")
				valueArgs = append(valueArgs, entry.playlistID, entry.songID, entry.position)
			}
			query := "INSERT INTO playlist_songs (playlist_id, song_id, position) VALUES " +
				strings.Join(valueStrings, ", ")
			if _, err := dbtx.ExecContext(ctx, query, valueArgs...); err != nil {
				return fmt.Errorf("batch insert playlist songs: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	response.Total = len(response.Playlists)
	return response, nil
}

// CountByCoverPath 统计 playlists 中等于该 cover_path 的总行数。
func (r *PlaylistRepository) CountByCoverPath(ctx context.Context, coverPath string) (int, error) {
	if coverPath == "" {
		return 0, nil
	}
	n, err := r.queries.CountPlaylistsByCoverPath(ctx, coverPath)
	if err != nil {
		return 0, fmt.Errorf("count playlists by cover_path: %w", err)
	}
	return int(n), nil
}

// runInTx 在底层为 *sql.DB 时自启动事务并把 *sql.Tx + 绑定后的 *sqlc.Queries 交给 fn；
// 底层已是 *sql.Tx 时直接复用，不嵌套事务。
func (r *PlaylistRepository) runInTx(ctx context.Context, fn func(sqlc.DBTX, *sqlc.Queries) error) error {
	if sqlDB, ok := r.db.(*sql.DB); ok {
		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()
		if err := fn(tx, r.queries.WithTx(tx)); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}
		committed = true
		return nil
	}
	return fn(r.db, r.queries)
}

// playlistSelectBuilder 给 List 用的 squirrel SELECT 模板，
// 字段顺序与 scanPlaylistRow 保持一致。
func playlistSelectBuilder() sq.SelectBuilder {
	return sq.Select(
		"p.id", "p.type", "p.name", "p.description",
		"p.cover_path", "p.cover_url", "p.labels",
		"p.sort_by", "p.sort_order", "p.pinned_at",
		"p.created_at", "p.updated_at",
		"COALESCE(cnt.song_count, 0) AS song_count",
		"COALESCE(cnt.remote_count, 0) AS remote_count",
	).From("playlists p").
		// 子查询 JOIN songs 只为多算一个 remote_count（网络歌单徽标用）。
		// song_count 语义不受影响：foreign_keys(1) 已开 + playlist_songs.song_id
		// 是 ON DELETE CASCADE 外键，不存在指向已删歌曲的孤儿行。
		LeftJoin("(SELECT ps.playlist_id, COUNT(*) AS song_count, " +
			"SUM(CASE WHEN s.type = 'remote' THEN 1 ELSE 0 END) AS remote_count " +
			"FROM playlist_songs ps JOIN songs s ON s.id = ps.song_id " +
			"GROUP BY ps.playlist_id) cnt ON p.id = cnt.playlist_id")
}

func applyPlaylistFilter(sb sq.SelectBuilder, filter *PlaylistFilter, prefix string) sq.SelectBuilder {
	if filter.Type != "" {
		sb = sb.Where(sq.Eq{prefix + "type": filter.Type})
	}
	for _, label := range filter.Labels {
		sb = sb.Where(fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%slabels) WHERE value = ?)", prefix), label)
	}
	for _, label := range filter.ExcludeLabels {
		sb = sb.Where(fmt.Sprintf("NOT EXISTS (SELECT 1 FROM json_each(%slabels) WHERE value = ?)", prefix), label)
	}
	if filter.Keyword != "" {
		kw := "%" + filter.Keyword + "%"
		sb = sb.Where(sq.Or{
			sq.Like{prefix + "name": kw},
			sq.Like{prefix + "description": kw},
		})
	}
	// SongSource 是 EXISTS 语义（含该来源的歌曲即命中），故混合歌单同时命中
	// remote 与 local，空歌单两者都不命中。
	//
	// 引用外层歌单 id 时**必须**带表限定：子查询里 join 了 songs，裸 `id` 会被
	// SQLite 判为 ambiguous column name（songs 也有 id 列）。prefix 为 "p."（List，
	// 带 JOIN 且表有别名）时用 "p."，为 ""（Count，From("playlists") 无别名）时
	// 必须补成 "playlists."，不能直接用空前缀。
	if filter.SongSource != "" {
		outer := prefix
		if outer == "" {
			outer = "playlists."
		}
		sb = sb.Where(fmt.Sprintf(
			"EXISTS (SELECT 1 FROM playlist_songs ps JOIN songs s ON s.id = ps.song_id "+
				"WHERE ps.playlist_id = %sid AND s.type = ?)", outer), filter.SongSource)
	}
	return sb
}

func scanPlaylistRow(scanner interface {
	Scan(dest ...any) error
}) (*models.Playlist, error) {
	p := &models.Playlist{}
	var labelsJSON sql.NullString
	var pinnedAt sql.NullTime
	var songCount int64
	var remoteCount int64
	if err := scanner.Scan(
		&p.ID, &p.Type, &p.Name, &p.Description,
		&p.CoverPath, &p.CoverURL, &labelsJSON,
		&p.SortBy, &p.SortOrder, &pinnedAt,
		&p.CreatedAt, &p.UpdatedAt, &songCount, &remoteCount,
	); err != nil {
		return nil, fmt.Errorf("scan playlist: %w", err)
	}
	p.Labels = parseLabelsJSON(labelsJSON)
	p.SongCount = int(songCount)
	p.RemoteCount = int(remoteCount)
	if pinnedAt.Valid {
		p.PinnedAt = &pinnedAt.Time
	}
	return p, nil
}

func playlistRowToModel(row sqlc.GetPlaylistByIDRow) *models.Playlist {
	p := &models.Playlist{
		ID:          row.ID,
		Type:        row.Type,
		Name:        row.Name,
		Description: row.Description,
		CoverPath:   row.CoverPath,
		CoverURL:    row.CoverUrl,
		Labels:      parseLabelsJSON(sql.NullString{String: row.Labels, Valid: row.Labels != ""}),
		SortBy:      row.SortBy,
		SortOrder:   row.SortOrder,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		SongCount:   int(row.SongCount),
	}
	if row.PinnedAt.Valid {
		p.PinnedAt = &row.PinnedAt.Time
	}
	return p
}

func parseLabelsJSON(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return []string{}
	}
	var labels []string
	if err := json.Unmarshal([]byte(s.String), &labels); err != nil {
		return []string{}
	}
	return labels
}

// generateSmartPlaylistNames 根据目录路径列表生成智能歌单名称和描述。
// 返回 dir -> name, dir -> description 两个映射。
func generateSmartPlaylistNames(dirs []string) (nameMap, descMap map[string]string) {
	nameMap = make(map[string]string, len(dirs))
	descMap = make(map[string]string, len(dirs))
	if len(dirs) == 0 {
		return
	}

	musicRoot := findCommonPathPrefix(dirs)

	infos := make([]playlistDirInfo, 0, len(dirs))
	for _, dir := range dirs {
		relPath := strings.TrimPrefix(dir, musicRoot)
		relPath = strings.TrimPrefix(relPath, "/")
		infos = append(infos, playlistDirInfo{
			dir:      dir,
			relPath:  relPath,
			baseName: filepath.Base(dir),
		})
	}

	baseNameGroups := make(map[string][]int)
	for i, info := range infos {
		baseNameGroups[info.baseName] = append(baseNameGroups[info.baseName], i)
	}

	for _, info := range infos {
		relParent := ""
		if info.relPath != "" && info.relPath != info.baseName {
			relParent = filepath.ToSlash(filepath.Dir(info.relPath))
			if relParent == "." {
				relParent = ""
			}
		}
		descMap[info.dir] = relParent

		group := baseNameGroups[info.baseName]
		if len(group) == 1 {
			nameMap[info.dir] = info.baseName
		} else {
			nameMap[info.dir] = disambiguateName(info, infos, group)
		}
	}

	// 边界：歌曲直接在音乐根目录（relPath 为空）。
	for _, info := range infos {
		if info.relPath == "" || info.relPath == "." {
			nameMap[info.dir] = filepath.Base(info.dir)
			descMap[info.dir] = ""
		}
	}
	return
}

func findCommonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return paths[0]
	}
	splitPath := func(p string) []string {
		return strings.Split(filepath.ToSlash(p), "/")
	}
	firstParts := splitPath(paths[0])
	commonLen := len(firstParts)
	for _, p := range paths[1:] {
		parts := splitPath(p)
		minLen := commonLen
		if len(parts) < minLen {
			minLen = len(parts)
		}
		newCommon := 0
		for i := 0; i < minLen; i++ {
			if firstParts[i] == parts[i] {
				newCommon++
			} else {
				break
			}
		}
		commonLen = newCommon
	}
	if commonLen == 0 {
		return ""
	}
	return strings.Join(firstParts[:commonLen], "/")
}

type playlistDirInfo struct {
	dir      string
	relPath  string
	baseName string
}

func disambiguateName(target playlistDirInfo, allInfos []playlistDirInfo, groupIndices []int) string {
	relParts := strings.Split(filepath.ToSlash(target.relPath), "/")
	if len(relParts) <= 1 {
		return target.relPath
	}
	parentParts := relParts[:len(relParts)-1]

	for depth := 1; depth <= len(parentParts); depth++ {
		startIdx := len(parentParts) - depth
		suffix := strings.Join(parentParts[startIdx:], "/")
		candidate := target.baseName + " - " + suffix

		unique := true
		for _, idx := range groupIndices {
			other := allInfos[idx]
			if other.dir == target.dir {
				continue
			}
			otherParts := strings.Split(filepath.ToSlash(other.relPath), "/")
			if len(otherParts) <= 1 {
				continue
			}
			otherParentParts := otherParts[:len(otherParts)-1]
			if depth <= len(otherParentParts) {
				otherStart := len(otherParentParts) - depth
				otherSuffix := strings.Join(otherParentParts[otherStart:], "/")
				if otherSuffix == suffix {
					unique = false
					break
				}
			}
		}

		if unique {
			return candidate
		}
	}
	return target.relPath
}

// resolveAutoCreatedName 把候选名解析到一个未占用的名字，
// 冲突则追加 " (自动)" / " (自动 2)" 后缀。调用方负责把返回的名字加入 existing。
func resolveAutoCreatedName(candidate string, existing map[string]struct{}) string {
	if _, conflict := existing[candidate]; !conflict {
		return candidate
	}
	suffixed := candidate + " (自动)"
	if _, conflict := existing[suffixed]; !conflict {
		return suffixed
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s (自动 %d)", candidate, n)
		if _, conflict := existing[cand]; !conflict {
			return cand
		}
	}
}

// pickDirCover 优先使用目录下的外部封面图片，找不到再回退到 pickSongCover。
func pickDirCover(dir string, songIDs []int64, songIDToSong map[int64]*models.Song, coverStoragePath string) (string, string) {
	if coverStoragePath != "" {
		if extPath, err := fileutil.FindExternalCover(dir); err == nil && extPath != "" {
			if savedPath, err := fileutil.SaveExternalCover(extPath, coverStoragePath); err == nil && savedPath != "" {
				return savedPath, ""
			}
		}
	}
	return pickSongCover(songIDs, songIDToSong)
}

// pickSongCover 从歌曲 ID 列表中确定性地选第一首有封面的，
// 返回 (CoverPath, CoverURL)，本地封面优先。
// 调用方在传入前已对 songIDs 做确定性排序（CUE 按 cue_track_index，
// 目录按数字前缀/标题），因此跨扫描结果稳定，不会让自动歌单封面随机跳变。
func pickSongCover(songIDs []int64, songIDToSong map[int64]*models.Song) (string, string) {
	for _, id := range songIDs {
		song, ok := songIDToSong[id]
		if !ok {
			continue
		}
		if song.CoverPath != "" || song.CoverURL != "" {
			return song.CoverPath, song.CoverURL
		}
	}
	return "", ""
}

// extractFirstNumber 提取字符串中第一段连续数字，
// 与 Flutter 前端 _extractFirstNumber 行为保持一致。
func extractFirstNumber(s string) (int, bool) {
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			n, err := strconv.Atoi(s[start:i])
			return n, err == nil
		}
	}
	if start >= 0 {
		n, err := strconv.Atoi(s[start:])
		return n, err == nil
	}
	return 0, false
}

// lessSongByNumberThenTitle 复刻 Flutter 端按数字前缀排序的规则：
// 双方都有数字 → 数值小者在前，相等回退到标题；只有一方有 → 有数字者在前；
// 都没有 → 标题不区分大小写排序。
func lessSongByNumberThenTitle(a, b *models.Song) bool {
	numA, okA := extractFirstNumber(a.Title)
	numB, okB := extractFirstNumber(b.Title)
	switch {
	case okA && okB:
		if numA != numB {
			return numA < numB
		}
		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	case okA:
		return true
	case okB:
		return false
	default:
		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	}
}
