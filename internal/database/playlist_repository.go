package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"songloft/internal/database/sqlc"
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

// Update 更新歌单。改名冲突返回 models.ErrPlaylistNameConflict，找不到返回 ErrNotFound。
func (r *PlaylistRepository) Update(ctx context.Context, playlist *models.Playlist) error {
	labelsJSON, err := json.Marshal(playlist.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	now := time.Now()
	return r.runInTx(ctx, func(dbtx sqlc.DBTX, q *sqlc.Queries) error {
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
		return nil
	})
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
func (r *PlaylistRepository) AutoCreate(ctx context.Context, includeSubdirs bool) (*models.AutoCreatePlaylistsResponse, error) {
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

	dirToSongs := make(map[string][]int64)
	for _, song := range songs {
		if song.FilePath == "" {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(song.FilePath))
		dirToSongs[dir] = append(dirToSongs[dir], song.ID)
		if includeSubdirs {
			parent := filepath.Dir(dir)
			for parent != "." && parent != "/" && parent != dir {
				parent = filepath.ToSlash(parent)
				dirToSongs[parent] = append(dirToSongs[parent], song.ID)
				parent = filepath.Dir(parent)
			}
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
		// 单条 SQL 批量删除旧的自动创建歌单（CASCADE 自动删除关联的 playlist_songs）
		if _, err := dbtx.ExecContext(ctx,
			`DELETE FROM playlists WHERE EXISTS (SELECT 1 FROM json_each(labels) WHERE value = ?)`,
			models.PlaylistLabelAutoCreated,
		); err != nil {
			return fmt.Errorf("delete old auto-created playlists: %w", err)
		}

		// 收集剩余歌单的现有名字（非 auto_created：用户手动建/内置歌单），用于消歧。
		// 同名约束在 Create 里强制；auto-create 走直接 INSERT 绕过了检查，必须自己消歧。
		existingNamesList, err := q.ListAllPlaylistNames(ctx)
		if err != nil {
			return fmt.Errorf("load existing playlist names: %w", err)
		}
		existingNames := make(map[string]struct{}, len(existingNamesList))
		for _, name := range existingNamesList {
			existingNames[name] = struct{}{}
		}

		for dir, songIDs := range dirToSongs {
			// 按数字前缀排序歌曲，与 Flutter 端展示顺序一致。
			sort.SliceStable(songIDs, func(i, j int) bool {
				return lessSongByNumberThenTitle(songIDToSong[songIDs[i]], songIDToSong[songIDs[j]])
			})

			coverPath, coverURL := pickRandomSongCover(songIDs, songIDToSong)
			playlistName := resolveAutoCreatedName(nameMap[dir], existingNames)
			existingNames[playlistName] = struct{}{}

			playlistID, err := q.InsertAutoCreatedPlaylist(ctx, sqlc.InsertAutoCreatedPlaylistParams{
				Type:        models.PlaylistTypeNormal,
				Name:        playlistName,
				Description: descMap[dir],
				CoverPath:   coverPath,
				CoverUrl:    coverURL,
				Labels:      labelsStr,
			})
			if err != nil {
				return fmt.Errorf("create playlist %s: %w", dir, err)
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
				Name:       playlistName,
				SongCount:  len(songIDs),
			})
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
		"p.created_at", "p.updated_at",
		"COALESCE(cnt.song_count, 0) AS song_count",
	).From("playlists p").
		LeftJoin("(SELECT playlist_id, COUNT(*) AS song_count FROM playlist_songs GROUP BY playlist_id) cnt ON p.id = cnt.playlist_id")
}

func applyPlaylistFilter(sb sq.SelectBuilder, filter *PlaylistFilter, prefix string) sq.SelectBuilder {
	if filter.Type != "" {
		sb = sb.Where(sq.Eq{prefix + "type": filter.Type})
	}
	for _, label := range filter.Labels {
		sb = sb.Where(fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%slabels) WHERE value = ?)", prefix), label)
	}
	if filter.Keyword != "" {
		kw := "%" + filter.Keyword + "%"
		sb = sb.Where(sq.Or{
			sq.Like{prefix + "name": kw},
			sq.Like{prefix + "description": kw},
		})
	}
	return sb
}

func scanPlaylistRow(scanner interface {
	Scan(dest ...any) error
}) (*models.Playlist, error) {
	p := &models.Playlist{}
	var labelsJSON sql.NullString
	var songCount int64
	if err := scanner.Scan(
		&p.ID, &p.Type, &p.Name, &p.Description,
		&p.CoverPath, &p.CoverURL, &labelsJSON,
		&p.CreatedAt, &p.UpdatedAt, &songCount,
	); err != nil {
		return nil, fmt.Errorf("scan playlist: %w", err)
	}
	p.Labels = parseLabelsJSON(labelsJSON)
	p.SongCount = int(songCount)
	return p, nil
}

func playlistRowToModel(row sqlc.GetPlaylistByIDRow) *models.Playlist {
	return &models.Playlist{
		ID:          row.ID,
		Type:        row.Type,
		Name:        row.Name,
		Description: row.Description,
		CoverPath:   row.CoverPath,
		CoverURL:    row.CoverUrl,
		Labels:      parseLabelsJSON(sql.NullString{String: row.Labels, Valid: row.Labels != ""}),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		SongCount:   int(row.SongCount),
	}
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

// pickRandomSongCover 从歌曲 ID 列表中随机选一个有封面的，
// 返回 (CoverPath, CoverURL)，本地封面优先。
func pickRandomSongCover(songIDs []int64, songIDToSong map[int64]*models.Song) (string, string) {
	candidates := make([]int64, 0, len(songIDs))
	for _, id := range songIDs {
		song, ok := songIDToSong[id]
		if !ok {
			continue
		}
		if song.CoverPath != "" || song.CoverURL != "" {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return "", ""
	}
	s := songIDToSong[candidates[rand.Intn(len(candidates))]]
	return s.CoverPath, s.CoverURL
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
