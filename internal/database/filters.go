package database

import (
	"strings"

	sq "github.com/Masterminds/squirrel"
)

// ConfigFilter 配置过滤条件
type ConfigFilter struct {
	Keyword string
	Limit   int
	Offset  int
	OrderBy string
	Order   string
}

// SongFilter 歌曲过滤条件
type SongFilter struct {
	Type       string
	Keyword    string
	PathPrefix string
	// 标签分类精确过滤（空字符串表示不过滤该维度）。
	Genre    string
	Artist   string
	Album    string
	Language string
	Style    string
	Year     int // 精确年份，0 表示不过滤
	// DecadeStart 年代起始年（如 1990 表示 1990-1999）；0 表示不过滤。
	DecadeStart int
	// TagID 按自定义标签 ID 过滤（0 表示不过滤）。
	TagID int64
	// TagName 按自定义标签名模糊匹配过滤歌曲（空表示不过滤）。
	TagName string
	// MinRating 评级下限（含），0 表示不过滤；用于「≥ N 星」筛选。
	// 评级值域为 0-100，0 恰好与「不过滤」语义重合（≥0 等于全量），故用 int 即可。
	MinRating int
	// ExactRating 评级精确匹配；nil 表示不过滤。指针语义避免与「0=未评级」
	// 的合法取值冲突（Go 零值陷阱）——0 指针表示只要未评级歌曲。
	ExactRating *int
	// ExcludePlaylistLabels 排除属于「带这些 label 的歌单」的歌曲。
	// 典型用途：排除隐藏歌单（label=hidden）里的歌，让主歌曲列表不显示它们。
	ExcludePlaylistLabels []string
	Limit                 int
	Offset                int
	OrderBy               string
	Order                 string
}

// PlaylistFilter 歌单过滤条件
type PlaylistFilter struct {
	Type          string
	Labels        []string
	ExcludeLabels []string
	Keyword       string
	// SongSource 按「歌单内歌曲的来源」过滤，取值为 songs.type：
	// remote=歌单内含网络歌曲（即「网络歌单」），local=含本地歌曲（即「本地歌单」），空=不过滤。
	// 语义是 EXISTS 而非「全部是」：混合歌单两边都命中，空歌单两边都不命中
	// （来源未知，归任何一边都是编的）。歌单本身没有来源字段，只能这样反推（songloft-org/songloft#445）。
	SongSource string
	Limit      int
	Offset     int
	OrderBy    string
	Order      string
}

// PlaylistSongFilter 歌单歌曲过滤/排序条件
type PlaylistSongFilter struct {
	Keyword string
	OrderBy string
	Order   string
	Limit   int
	Offset  int
}

// FacetFilter 标签分类聚合（facet）的过滤/排序/分页条件。
type FacetFilter struct {
	// Type applies the same query-only song type filter as SongFilter.
	// "music" matches local and remote songs outside audiobook playlists.
	Type string
	// Keyword 对取值做模糊搜索（空表示不过滤）。
	Keyword string
	// OrderBy 排序维度："count"（按歌曲数，默认）或 "name"（按取值名）。
	OrderBy string
	// Order 排序方向 asc/desc；为空时按 OrderBy 取默认（count→desc，name→asc）。
	Order  string
	Limit  int
	Offset int
}

// TokenFilter Token 过滤条件
type TokenFilter struct {
	TokenType string
	IsActive  *bool
	Keyword   string
	Limit     int
	Offset    int
	OrderBy   string
	Order     string
}

// 排序字段白名单：防止 SQL 注入。
// 调用方传入的 OrderBy 必须在白名单内，否则回退到默认排序。
var (
	songOrderWhitelist = map[string]struct{}{
		"id": {}, "title": {}, "artist": {}, "album": {},
		"duration": {}, "added_at": {}, "updated_at": {},
		"file_modified_at": {}, "year": {}, "genre": {}, "file_size": {},
		"rating": {},
	}
	playlistOrderWhitelist = map[string]struct{}{
		"id": {}, "name": {}, "position": {},
		"created_at": {}, "updated_at": {},
	}
	configOrderWhitelist = map[string]struct{}{
		"id": {}, "key": {}, "updated_at": {},
	}
	tokenOrderWhitelist = map[string]struct{}{
		"id": {}, "token_type": {}, "expires_at": {}, "created_at": {},
	}
	playlistSongOrderWhitelist = map[string]struct{}{
		"position": {}, "added_at": {}, "title": {},
		"artist": {}, "album": {}, "duration": {}, "updated_at": {},
		"file_modified_at": {}, "file_size": {},
	}
	playlistSongOrderColumn = map[string]string{
		"position":         "ps.position",
		"added_at":         "ps.added_at",
		"title":            "s.title",
		"artist":           "s.artist",
		"album":            "s.album",
		"duration":         "s.duration",
		"updated_at":       "s.updated_at",
		"file_modified_at": "s.file_modified_at",
		"file_size":        "s.file_size",
	}
	// songFacetColumn 把 facet 维度映射到固定的 SQL 列名/表达式。
	// 仅使用映射值拼接列名（绝不拼用户输入），防 SQL 注入 —— 同 playlistSongOrderColumn 范式。
	songFacetColumn = map[string]string{
		"genre":    "genre",
		"artist":   "artist",
		"album":    "album",
		"language": "language",
		"style":    "style",
		"year":     "year",
		"decade":   "(year / 10) * 10",
		"rating":   "rating",
	}
	// songNameColumn 把 /songs/names 的 field 映射到固定 SQL 列名。
	// 仅使用映射值拼接列名（绝不拼用户输入），防 SQL 注入 —— 同 songFacetColumn 范式。
	songNameColumn = map[string]string{
		"title":  "title",
		"artist": "artist",
	}
)

// IsSongNameField 判断给定字符串是否为受支持的 /songs/names 维度（title/artist）。
// 对外导出以便 handlers 复用同一份维度清单，避免各层各写一份枚举而漂移。
func IsSongNameField(field string) bool {
	_, ok := songNameColumn[field]
	return ok
}

// songFacetTagField 是 tag 维度的 field 名。它不映射到 songs 的单列，因此不在
// songFacetColumn 里，而是由 SongRepository.ListFacet/CountFacet 走专门的 join 分支。
// 用常量集中，避免 handlers / repository / play_history 各处硬编码字符串而漂移。
const songFacetTagField = "tag"

// songFacetArtistField 是 artist 维度的 field 名。它走 song_artists 关联表（多值拆分），
// 不走 songs.artist 单列，故同样作为特殊分支处理（见 buildArtistFacetSelect）。
const songFacetArtistField = "artist"

// IsSongFacetField 判断给定字符串是否为受支持的歌曲分面维度。
// 对外导出以便 models / handlers 复用同一份维度清单，避免各层各写一份枚举而漂移。
func IsSongFacetField(field string) bool {
	if field == songFacetTagField || field == songFacetArtistField {
		return true
	}
	_, ok := songFacetColumn[field]
	return ok
}

// facetBaseCond 返回某 facet 维度「取值非空」的基础过滤条件。
// year/decade 用 year>0；rating 用 rating>0（0 表示未评级，不参与聚合）；
// 文本维度用 <col> != ”。
func facetBaseCond(field, col string) sq.Sqlizer {
	if field == "year" || field == "decade" {
		return sq.Gt{"year": 0}
	}
	if field == "rating" {
		return sq.Gt{"rating": 0}
	}
	return sq.NotEq{col: ""}
}

// applyFacetOrder 对 facet 聚合结果应用排序。
// count→按歌曲数（默认 DESC，附带 value ASC 稳定次序）；name→按取值名（默认 ASC）。
func applyFacetOrder(sb sq.SelectBuilder, f *FacetFilter) sq.SelectBuilder {
	orderBy := "count"
	order := ""
	if f != nil {
		if f.OrderBy != "" {
			orderBy = f.OrderBy
		}
		order = f.Order
	}
	if orderBy == "name" {
		dir := "ASC"
		if strings.EqualFold(order, "DESC") {
			dir = "DESC"
		}
		return sb.OrderBy("value " + dir)
	}
	// 默认按 count
	dir := "DESC"
	if strings.EqualFold(order, "ASC") {
		dir = "ASC"
	}
	return sb.OrderBy("count "+dir, "value ASC")
}

// applyOrder 把 orderBy/order 加到 squirrel SELECT 上。
// orderBy 不在白名单时退化到 defaultOrder（已含 ASC/DESC）。
// tablePrefix 用于带 JOIN 的查询（如 "p."），无前缀传 ""。
func applyOrder(sb sq.SelectBuilder, orderBy, order, defaultOrder string, whitelist map[string]struct{}, tablePrefix string) sq.SelectBuilder {
	if orderBy == "" {
		return sb.OrderBy(defaultOrder)
	}
	if _, ok := whitelist[orderBy]; !ok {
		return sb.OrderBy(defaultOrder)
	}
	dir := "ASC"
	if strings.EqualFold(order, "DESC") {
		dir = "DESC"
	}
	return sb.OrderBy(tablePrefix + orderBy + " " + dir)
}

// applyPlaylistSongOrder 对歌单歌曲查询应用排序。
// 与 applyOrder 不同，使用列名映射表将用户传入的字段名转换为带表别名的列名。
func applyPlaylistSongOrder(sb sq.SelectBuilder, orderBy, order string) sq.SelectBuilder {
	col := "ps.position"
	if orderBy != "" {
		if mapped, ok := playlistSongOrderColumn[orderBy]; ok {
			col = mapped
		}
	}
	dir := "ASC"
	if strings.EqualFold(order, "DESC") {
		dir = "DESC"
	}
	if col == "s.title" || col == "s.artist" || col == "s.album" {
		col = "songloft_text_sort_key(" + col + ")"
	}
	// 二级排序键：主排序列存在等值行时（如批量加入歌单的 ps.added_at 全相同），
	// 以 ps.position 同方向兜底，保证 ASC/DESC 总能产生可见的不同顺序
	// （songloft-org/songloft#431）。主排序已是 ps.position 时二级键冗余但无害。
	return sb.OrderBy(col + " " + dir + ", ps.position " + dir)
}

// applyPagination 把 limit/offset 加到 squirrel SELECT 上。limit<=0 视为不分页。
func applyPagination(sb sq.SelectBuilder, limit, offset int) sq.SelectBuilder {
	if limit <= 0 {
		return sb
	}
	sb = sb.Limit(uint64(limit))
	if offset > 0 {
		sb = sb.Offset(uint64(offset))
	}
	return sb
}
