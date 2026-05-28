package models

// 分页限制常量
const (
	// DefaultPaginationLimit 默认分页大小，用于普通列表查询
	DefaultPaginationLimit = 20

	// MaxPaginationLimit 最大分页限制，用于获取"所有"数据的场景
	// 选择 100000 的原因：
	// 1. 足够大以覆盖绝大多数用户的歌曲数量（普通用户通常只有几百到几千首歌）
	// 2. 避免无限制查询导致性能问题
	// 3. 用于批量操作场景（如自动创建歌单、批量刮削等）
	MaxPaginationLimit = 100000
)
