package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// 歌曲类型常量
const (
	TypeLocal  = "local"  // 本地歌曲
	TypeRemote = "remote" // 网络歌曲
	TypeRadio  = "radio"  // 电台
)

// 歌单类型常量
const (
	PlaylistTypeNormal = "normal" // 普通歌单
	PlaylistTypeRadio  = "radio"  // 电台歌单
)

// 歌词来源常量
const (
	LyricSourceFile     = "file"     // .lrc 文件
	LyricSourceEmbedded = "embedded" // 内嵌歌词
	LyricSourceURL      = "url"      // URL 延迟加载（lyric 字段存放获取路径）
	LyricSourceCached   = "cached"   // 从 URL 获取后缓存（lyric 字段存放歌词文本）
)

// 错误定义
var (
	ErrInvalidType     = errors.New("invalid type")
	ErrMissingTitle    = errors.New("missing title")
	ErrMissingFilePath = errors.New("missing file path for local song")
	ErrMissingURL      = errors.New("missing url for remote song or radio")
	ErrMissingName     = errors.New("missing name")
	ErrMissingPlaylist = errors.New("missing playlist id")
	ErrMissingSong     = errors.New("missing song id")
	ErrInvalidPosition = errors.New("invalid position")
	ErrMissingKey      = errors.New("missing key")
	ErrMissingValue    = errors.New("missing value")
	ErrInvalidSongType = errors.New("invalid song type for this playlist")

	// ErrPlaylistNameConflict 表示已存在同名歌单（不区分类型）
	ErrPlaylistNameConflict = errors.New("playlist with the same name already exists")
)

// 认证相关常量
const (
	TokenTypeAccess  = "access"  // Access Token 类型
	TokenTypeRefresh = "refresh" // Refresh Token 类型
)

// 认证相关错误
var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrInvalidTokenType   = errors.New("invalid token type")
)

// Song 歌曲/电台结构体
type Song struct {
	ID              int64   `json:"id" example:"1"`                                    // 歌曲ID
	Type            string  `json:"type" example:"local" enums:"local,remote,radio"`   // 歌曲类型：local/remote/radio
	Title           string  `json:"title" example:"夜曲"`                                // 标题
	Artist          string  `json:"artist" example:"周杰伦"`                              // 艺术家/歌手
	Album           string  `json:"album" example:"十一月的萧邦"`                            // 专辑名称
	Duration        float64 `json:"duration" example:"253.5"`                          // 播放时长（秒）
	FilePath        string  `json:"file_path" example:"/music/周杰伦/夜曲.mp3"`             // 本地文件路径
	URL             string  `json:"url" example:"https://example.com/song.mp3"`        // 网络地址
	CoverPath       string  `json:"-"`                                                 // 封面图片本地路径(内部使用,不暴露给客户端)
	CoverURL        string  `json:"cover_url" example:"https://example.com/cover.jpg"` // 封面图片URL
	Lyric           string  `json:"-"`                                                 // 歌词 LyricPayload JSON 文本(内部存储,不暴露给客户端);lyric_source=url 时为空,真正的 URL 在 LyricRemoteURL
	LyricSource     string  `json:"-"`                                                 // 歌词来源(内部使用,不暴露给客户端)
	LyricRemoteURL  string  `json:"-"`                                                 // lyric_source=url 时的原始插件 URL(内部使用,运行时由 LyricFetcher 拉取)
	LyricURL        string  `json:"lyric_url,omitempty"`                               // 歌词端点 URL(客户端唯一可见字段,指向 /api/v1/songs/{id}/lyric)
	FileSize        int64   `json:"file_size" example:"10485760"`                      // 文件大小（字节）
	Format          string  `json:"format" example:"mp3"`                              // 音频格式
	BitRate         int     `json:"bit_rate" example:"320"`                            // 比特率（kbps）
	SampleRate      int     `json:"sample_rate" example:"44100"`                       // 采样率（Hz）
	IsLive          bool    `json:"is_live" example:"false"`                           // 是否为直播流
	PluginEntryPath string  `json:"plugin_entry_path,omitempty" example:"my-source"`   // 音源插件 entryPath(网络歌曲)
	SourceData      string  `json:"source_data,omitempty"`                             // 音源元数据 JSON(给插件 music/url 接口用,opaque)
	DedupKey        string  `json:"dedup_key,omitempty"`                               // 去重 key(由插件定义,典型形态 "<platform>:<platform_id>");与 PluginEntryPath 组成 UNIQUE

	AddedAt   time.Time `json:"added_at" example:"2024-01-01T12:00:00Z"`   // 添加时间
	UpdatedAt time.Time `json:"updated_at" example:"2024-01-01T12:00:00Z"` // 最后更新时间
}

// IsPluginSourced 判断是否插件来源的歌曲
func (s *Song) IsPluginSourced() bool {
	return s.PluginEntryPath != "" && s.SourceData != ""
}

// PlaybackURL 返回客户端用的统一播放 URL(所有 type 都用同一形态)。
// 客户端只需要 setUrl(song.url),不需要判断 type。
// handler 内部按 type 分发到本地 ServeFile / Orchestrator / 直链下载 / 电台 302。
func (s *Song) PlaybackURL() string {
	if s.ID == 0 {
		return ""
	}
	return fmt.Sprintf("/api/v1/songs/%d/play", s.ID)
}

// CoverURLPath 返回客户端用的统一封面 URL。
// 有封面(本地或远程)时返回 /api/v1/songs/{id}/cover 端点,后端自动判断本地/远程
// 无封面时返回空字符串,避免客户端发起注定 404 的请求
func (s *Song) CoverURLPath() string {
	if s.ID == 0 {
		return ""
	}
	if s.CoverPath == "" && s.CoverURL == "" {
		return ""
	}
	return fmt.Sprintf("/api/v1/songs/%d/cover", s.ID)
}

// LyricURLPath 返回客户端用的统一歌词 URL。
// 有歌词时(无论来源):返回 /api/v1/songs/{id}/lyric 端点
// 无歌词时:返回空字符串
func (s *Song) LyricURLPath() string {
	if s.ID == 0 {
		return ""
	}
	// Lyric 字段(本地落库的 JSON payload)或 LyricRemoteURL(待拉取的插件 URL)任一非空,就返回歌词端点
	if s.Lyric != "" || s.LyricRemoteURL != "" {
		return fmt.Sprintf("/api/v1/songs/%d/lyric", s.ID)
	}
	return ""
}

// MarshalJSON 序列化时把 URL、CoverURL、LyricURL 字段统一覆盖为服务端端点。
// 这样所有 handler/list/playlist 接口返回的 song.url 都是 /api/v1/songs/{id}/play,
// 本地歌曲的 cover_url 是 /api/v1/songs/{id}/cover,网络歌曲保留原始 CoverURL。
// 有歌词时 lyric_url 是 /api/v1/songs/{id}/lyric,无歌词时为空。
// 客户端拿到后直接使用,无需关心 type、file_path、source_data、lyric_source 等内部细节。
// 原始 URL(remote 的插件接口路径、radio 的流地址)留在后端数据库,不暴露给客户端。
// 所有 type 的播放都通过 /api/v1/songs/{id}/play 统一分发。
func (s *Song) MarshalJSON() ([]byte, error) {
	type songAlias Song
	if s.ID != 0 {
		s.URL = s.PlaybackURL()
		s.CoverURL = s.CoverURLPath()
		s.LyricURL = s.LyricURLPath()
	}
	return json.Marshal((*songAlias)(s))
}

// Validate 验证歌曲数据有效性
func (s *Song) Validate() error {
	// 验证标题
	if s.Title == "" {
		return ErrMissingTitle
	}

	// 验证类型
	if s.Type != TypeLocal && s.Type != TypeRemote && s.Type != TypeRadio {
		return ErrInvalidType
	}

	// 根据类型验证必需字段
	switch s.Type {
	case TypeLocal:
		if s.FilePath == "" {
			return ErrMissingFilePath
		}
	case TypeRemote, TypeRadio:
		if s.URL == "" {
			return ErrMissingURL
		}
	}

	return nil
}

// IsLocal 判断是否为本地歌曲
func (s *Song) IsLocal() bool {
	return s.Type == TypeLocal
}

// IsRadio 判断是否为电台/广播
func (s *Song) IsRadio() bool {
	return s.Type == TypeRadio
}

// CoverURLPath 返回客户端用的统一封面 URL。
// 有封面(本地或远程)时返回 /api/v1/playlists/{id}/cover 端点,后端自动判断本地/远程
// 无封面时返回空字符串,避免客户端发起注定 404 的请求
func (p *Playlist) CoverURLPath() string {
	if p.ID == 0 {
		return ""
	}
	if p.CoverPath == "" && p.CoverURL == "" {
		return ""
	}
	return fmt.Sprintf("/api/v1/playlists/%d/cover", p.ID)
}

// MarshalJSON 序列化时把 CoverURL 字段统一覆盖为服务端端点。
// 这样所有歌单接口返回的 cover_url 都是 /api/v1/playlists/{id}/cover 或原始 CDN URL。
// 客户端拿到后直接使用,无需关心 cover_path 等内部细节。
func (p *Playlist) MarshalJSON() ([]byte, error) {
	type playlistAlias Playlist
	if p.ID != 0 {
		p.CoverURL = p.CoverURLPath()
	}
	return json.Marshal((*playlistAlias)(p))
}

// Playlist 歌单结构体
type Playlist struct {
	ID          int64     `json:"id" example:"1"`                                       // 歌单 ID
	Type        string    `json:"type" example:"normal" enums:"normal,radio"`           // 歌单类型：normal/radio
	Name        string    `json:"name" example:"我的最爱"`                                  // 歌单名称
	Description string    `json:"description" example:"收藏的经典歌曲"`                        // 歌单描述
	CoverPath   string    `json:"-"`                                                    // 封面图片本地路径(内部使用,不暴露给客户端)
	CoverURL    string    `json:"cover_url" example:"https://example.com/playlist.jpg"` // 封面图片 URL
	Labels      []string  `json:"labels" example:"[\"built_in\"]"`                      // 歌单标签，如 ["built_in"]
	SongCount   int       `json:"song_count" example:"10"`                              // 歌曲数量
	CreatedAt   time.Time `json:"created_at" example:"2024-01-01T12:00:00Z"`            // 创建时间
	UpdatedAt   time.Time `json:"updated_at" example:"2024-01-01T12:00:00Z"`            // 最后更新时间
}

// Validate 验证歌单数据有效性（用于创建场景，需要校验 type）
func (p *Playlist) Validate() error {
	// 验证名称
	if p.Name == "" {
		return ErrMissingName
	}

	// 验证类型
	if p.Type != PlaylistTypeNormal && p.Type != PlaylistTypeRadio {
		return ErrInvalidType
	}

	return nil
}

// ValidateForUpdate 验证歌单更新数据有效性（不校验 type，type 不允许修改）
func (p *Playlist) ValidateForUpdate() error {
	// 验证名称
	if p.Name == "" {
		return ErrMissingName
	}

	return nil
}

// CanAddSong 判断是否可以添加指定类型的歌曲
func (p *Playlist) CanAddSong(songType string) bool {
	switch p.Type {
	case PlaylistTypeNormal:
		// 普通歌单只能添加本地和网络歌曲
		return songType == TypeLocal || songType == TypeRemote
	case PlaylistTypeRadio:
		// 电台歌单只能添加电台/广播
		return songType == TypeRadio
	default:
		return false
	}
}

// PlaylistSong 歌单-歌曲关联结构体
type PlaylistSong struct {
	ID         int64     `json:"id" example:"1"`                          // 关联ID
	PlaylistID int64     `json:"playlist_id" example:"1"`                 // 所属歌单ID
	SongID     int64     `json:"song_id" example:"1"`                     // 歌曲/电台ID
	Position   int       `json:"position" example:"1"`                    // 位置顺序
	AddedAt    time.Time `json:"added_at" example:"2024-01-01T12:00:00Z"` // 添加时间
}

// Validate 验证歌单-歌曲关联数据有效性
func (ps *PlaylistSong) Validate() error {
	if ps.PlaylistID == 0 {
		return ErrMissingPlaylist
	}
	if ps.SongID == 0 {
		return ErrMissingSong
	}
	if ps.Position < 1 {
		return ErrInvalidPosition
	}
	return nil
}

// Config 配置结构体
type Config struct {
	ID        int64     `json:"id" example:"1"`                            // 配置ID
	Key       string    `json:"key" example:"music_path"`                  // 配置键
	Value     string    `json:"value" example:"{\"path\":\"/music\"}"`     // 配置值（JSON格式）
	UpdatedAt time.Time `json:"updated_at" example:"2024-01-01T12:00:00Z"` // 更新时间
}

// Validate 验证配置数据有效性
func (c *Config) Validate() error {
	if c.Key == "" {
		return ErrMissingKey
	}
	if c.Value == "" {
		return ErrMissingValue
	}
	return nil
}

// ErrorResponse 错误响应结构体
type ErrorResponse struct {
	Error  string `json:"error" example:"操作失败"`              // 错误信息
	Detail string `json:"detail,omitempty" example:"详细错误信息"` // 详细错误信息（可选）
}

// SuccessResponse 成功响应结构体
type SuccessResponse struct {
	Message string `json:"message" example:"操作成功"` // 成功信息
}

// ConfigFilter 配置过滤条件
type ConfigFilter struct {
	Keyword string // 关键词搜索（key）
	Limit   int    // 分页限制
	Offset  int    // 分页偏移
	OrderBy string // 排序字段
	Order   string // 排序方向：ASC/DESC
}

// RemoteVersionInfo 远程版本信息
type RemoteVersionInfo struct {
	Version           string `json:"version" example:"v1.0.0"`                                                                              // 版本号
	GitCommit         string `json:"git_commit" example:"abc123"`                                                                           // Git 提交哈希
	BuildTime         string `json:"build_time" example:"2024-01-01_12:00:00"`                                                              // 构建时间
	DownloadURLPrefix string `json:"download_url_prefix" example:"https://github.com/songloft-org/songloft/releases/download/v1.0.0/songloft"` // 下载地址前缀
	ReleaseNotes      string `json:"release_notes" example:"正式版本更新说明"`                                                                      // 发布说明
}

// UpgradeProgress 升级进度信息
type UpgradeProgress struct {
	Status      string `json:"status" example:"downloading" enums:"idle,downloading,testing,replacing,restarting,completed,failed"` // 状态
	Progress    int    `json:"progress" example:"50"`                                                                               // 进度百分比 (0-100)
	CurrentStep string `json:"current_step" example:"正在下载新版本..."`                                                                   // 当前步骤描述
	Error       string `json:"error,omitempty" example:"下载失败"`                                                                      // 错误信息（如有）
}

// 升级状态常量
const (
	UpgradeStatusIdle        = "idle"        // 空闲
	UpgradeStatusDownloading = "downloading" // 下载中
	UpgradeStatusTesting     = "testing"     // 测试中
	UpgradeStatusReplacing   = "replacing"   // 替换中
	UpgradeStatusResetting   = "resetting"   // 回退中
	UpgradeStatusRestarting  = "restarting"  // 重启中
	UpgradeStatusFailed      = "failed"      // 失败
)

// CreateConfigRequest 创建配置请求结构体
type CreateConfigRequest struct {
	Key   string `json:"key" example:"music_path" binding:"required"`              // 配置键
	Value string `json:"value" example:"{\"path\":\"/music\"}" binding:"required"` // 配置值（JSON 格式）
}

// UpdateConfigRequest 更新配置请求结构体
type UpdateConfigRequest struct {
	Value string `json:"value" example:"{\"path\":\"/new_music\"}" binding:"required"` // 配置值（JSON 格式）
}

// RefreshTokenRequest 刷新令牌请求结构体
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." binding:"required"` // 刷新令牌
}

// RevokeTokenRequest 撤销令牌请求结构体
type RevokeTokenRequest struct {
	Reason string `json:"reason" example:"用户主动登出"` // 撤销原因
}

// TokenInfo 令牌信息结构体
type TokenInfo struct {
	TokenID       string    `json:"token_id" example:"abc123"`                              // 令牌 ID
	TokenType     string    `json:"token_type" example:"access" enums:"access,refresh"`     // 令牌类型
	ClientInfo    string    `json:"client_info" example:"Mozilla/5.0 AppleWebKit/605.1.15"` // 客户端信息
	ExpiresAt     time.Time `json:"expires_at" example:"2024-01-08T12:00:00Z"`              // 过期时间
	CreatedAt     time.Time `json:"created_at" example:"2024-01-01T12:00:00Z"`              // 创建时间
	RevokedAt     time.Time `json:"revoked_at,omitempty" example:"2024-01-01T12:00:00Z"`    // 撤销时间
	RevokedBy     string    `json:"revoked_by,omitempty" example:"user"`                    // 撤销者
	RevokedReason string    `json:"revoked_reason,omitempty" example:"用户主动登出"`              // 撤销原因
}

// AuthToken 认证令牌结构体
type AuthToken struct {
	ID            int64     `json:"id" example:"1"`                                         // 记录 ID
	TokenID       string    `json:"token_id" example:"abc123"`                              // 令牌 ID
	TokenType     string    `json:"token_type" example:"access" enums:"access,refresh"`     // 令牌类型
	ClientInfo    string    `json:"client_info" example:"Mozilla/5.0 AppleWebKit/605.1.15"` // 客户端信息
	ExpiresAt     time.Time `json:"expires_at" example:"2024-01-08T12:00:00Z"`              // 过期时间
	RevokedAt     time.Time `json:"revoked_at,omitempty" example:"2024-01-01T12:00:00Z"`    // 撤销时间
	RevokedBy     string    `json:"revoked_by,omitempty" example:"user"`                    // 撤销者
	CreatedAt     time.Time `json:"created_at" example:"2024-01-01T12:00:00Z"`              // 创建时间
	RevokedReason string    `json:"revoked_reason,omitempty" example:"用户主动登出"`              // 撤销原因
}

// Music 音乐信息结构体（保留用于向后兼容）
// Deprecated: 使用 Song 代替
type Music struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	URL    string `json:"url"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" example:"admin" binding:"required"` // 用户名
	Password string `json:"password" example:"admin" binding:"required"` // 密码
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`  // Access Token
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."` // Refresh Token
	ExpiresIn    int64  `json:"expires_in" example:"604800"`                                     // Access Token 过期时间（秒）
	TokenType    string `json:"token_type" example:"Bearer"`                                     // Token 类型
}

// AutoCreatePlaylistsRequest 自动创建歌单请求结构体
type AutoCreatePlaylistsRequest struct {
	IncludeSubdirs bool `json:"include_subdirs" example:"false"` // 是否包含子目录
}

// PlaylistInfo 歌单信息结构体（用于自动创建歌单响应）
type PlaylistInfo struct {
	PlaylistID int64  `json:"playlist_id" example:"1"`   // 歌单 ID
	Name       string `json:"name" example:"/music/周杰伦"` // 歌单名称（完整路径）
	SongCount  int    `json:"song_count" example:"10"`   // 包含的歌曲数量
}

// AutoCreatePlaylistsResponse 自动创建歌单响应结构体
type AutoCreatePlaylistsResponse struct {
	Playlists []PlaylistInfo `json:"playlists"`         // 创建的歌单列表
	Total     int            `json:"total" example:"3"` // 创建的歌单总数
}

// 自动创建歌单相关常量
const (
	PlaylistLabelAutoCreated = "auto_created" // 自动创建歌单标签
	PlaylistLabelBuiltIn     = "built_in"     // 内置歌单标签（不可删除）
)

// BatchDeleteSongsRequest 批量删除歌曲请求
type BatchDeleteSongsRequest struct {
	IDs []int64 `json:"ids" example:"1"` // 要删除的歌曲 ID 列表
}

// BatchDeleteSongsResponse 批量删除歌曲响应
type BatchDeleteSongsResponse struct {
	Deleted int `json:"deleted" example:"3"` // 实际删除的数量
}

// BatchDeletePlaylistsRequest 批量删除歌单请求
type BatchDeletePlaylistsRequest struct {
	IDs []int64 `json:"ids" example:"1"` // 要删除的歌单 ID 列表
}

// BatchDeletePlaylistsResponse 批量删除歌单响应
type BatchDeletePlaylistsResponse struct {
	Deleted int `json:"deleted" example:"3"` // 实际删除的数量
}

// JSPluginStatus JS 插件状态枚举
type JSPluginStatus string

const (
	JSPluginStatusActive   JSPluginStatus = "active"   // 激活状态
	JSPluginStatusInactive JSPluginStatus = "inactive" // 未激活状态
	JSPluginStatusError    JSPluginStatus = "error"    // 错误状态
)

// JSPlugin JS 插件模型
type JSPlugin struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	Description    string         `json:"description"`
	Author         string         `json:"author"`
	Homepage       string         `json:"homepage,omitempty"`
	License        string         `json:"license,omitempty"`
	EntryPath      string         `json:"entry_path"` // 路由前缀（如 "myplugin"）
	Main           string         `json:"main"`       // 入口文件路径（如 "main.js"）
	MinHostVersion string         `json:"min_host_version,omitempty"`
	Permissions    []string       `json:"permissions"` // 权限列表
	UpdateURL      string         `json:"update_url,omitempty"`
	DownloadURL    string         `json:"download_url,omitempty"`
	Status         JSPluginStatus `json:"status"`
	ZipHash        string         `json:"zip_hash,omitempty"`   // ZIP 文件 SHA256
	EntryHash      string         `json:"entry_hash,omitempty"` // main.js/main.jsc 内容 SHA256
	FileModTime    string         `json:"file_mod_time,omitempty"`
	FilePath       string         `json:"file_path"` // ZIP 文件相对路径
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
