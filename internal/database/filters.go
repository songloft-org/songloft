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
	Type    string
	Keyword string
	Limit   int
	Offset  int
	OrderBy string
	Order   string
}

// PlaylistFilter 歌单过滤条件
type PlaylistFilter struct {
	Type    string
	Labels  []string
	Keyword string
	Limit   int
	Offset  int
	OrderBy string
	Order   string
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
)

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
