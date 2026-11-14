package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync/atomic"
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
	LyricSourceScraped  = "scraped"  // 由歌词提供者插件自动搜索获取
	LyricSourceManual   = "manual"   // 用户手动调整后的歌词，scanner 重扫不再覆盖
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

	// ErrBuiltInPlaylist 表示对内置歌单执行了不允许的操作
	ErrBuiltInPlaylist = errors.New("cannot modify built-in playlist")
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
	ID                  int64   `json:"id" example:"1"`                                    // 歌曲ID
	Type                string  `json:"type" example:"local" enums:"local,remote,radio"`   // 歌曲类型：local/remote/radio
	Title               string  `json:"title" example:"夜曲"`                                // 标题
	Artist              string  `json:"artist" example:"周杰伦"`                              // 艺术家/歌手
	Album               string  `json:"album" example:"十一月的萧邦"`                            // 专辑名称
	Year                int     `json:"year" example:"2005"`                               // 发行年份
	Genre               string  `json:"genre" example:"Pop"`                               // 流派
	Language            string  `json:"language,omitempty" example:"国语"`                   // 语种
	Style               string  `json:"style,omitempty" example:"抒情"`                      // 风格
	Duration            float64 `json:"duration" example:"253.5"`                          // 播放时长（秒）
	FilePath            string  `json:"file_path" example:"/music/周杰伦/夜曲.mp3"`             // 本地文件路径
	CachePath           string  `json:"-"`                                                 // 缓存文件路径（内部使用，不暴露给客户端）
	URL                 string  `json:"url" example:"https://example.com/song.mp3"`        // 网络地址
	CoverPath           string  `json:"-"`                                                 // 封面图片本地路径(内部使用,不暴露给客户端)
	CoverURL            string  `json:"cover_url" example:"https://example.com/cover.jpg"` // 封面图片URL
	Lyric               string  `json:"-"`                                                 // 歌词 LyricPayload JSON 文本(内部存储,不暴露给客户端);lyric_source=url 时为空,真正的 URL 在 LyricRemoteURL
	LyricSource         string  `json:"-"`                                                 // 歌词来源(内部使用,不暴露给客户端)
	LyricRemoteURL      string  `json:"lyric_remote_url,omitempty"`                        // lyric_source=url 时的原始 URL(运行时由 LyricFetcher 拉取)
	LyricURL            string  `json:"lyric_url,omitempty"`                               // 歌词端点 URL(客户端唯一可见字段,指向 /api/v1/songs/{id}/lyric)
	FileSize            int64   `json:"file_size" example:"10485760"`                      // 文件大小（字节）
	Format              string  `json:"format" example:"mp3"`                              // 音频格式
	BitRate             int     `json:"bit_rate" example:"320"`                            // 比特率（kbps）
	SampleRate          int     `json:"sample_rate" example:"44100"`                       // 采样率（Hz）
	IsLive              bool    `json:"is_live" example:"false"`                           // 是否为直播流
	IsVideo             bool    `json:"is_video" example:"false"`                          // 是否含真实视频轨(扫描时 ffprobe 探测,排除封面);客户端据此渲染画面/选择投屏 mime
	PluginEntryPath     string  `json:"plugin_entry_path,omitempty" example:"my-source"`   // 音源插件 entryPath(网络歌曲)
	SourceData          string  `json:"source_data,omitempty"`                             // 音源元数据 JSON(给插件 music/url 接口用,opaque)
	DedupKey            string  `json:"dedup_key,omitempty"`                               // 去重 key(由插件定义,典型形态 "<platform>:<platform_id>");与 PluginEntryPath 组成 UNIQUE
	SourceURL           string  `json:"source_url,omitempty"`                              // 原始音源 URL(仅 JSON 输出,radio/remote 类型返回原始流地址供编辑使用)
	SourceCoverURL      string  `json:"source_cover_url,omitempty"`                        // 原始封面 URL(仅 JSON 输出,CoverURL 非空时保留原始值供编辑使用)
	Fingerprint         string  `json:"fingerprint,omitempty"`                             // 音频指纹(Chromaprint)
	FingerprintDuration float64 `json:"fingerprint_duration,omitempty"`                    // 指纹对应音频时长
	ISRC                string  `json:"isrc,omitempty"`                                    // ISRC(国际标准录音编码)
	Track               string  `json:"track,omitempty" example:"3/12"`                    // 音轨号，可为 "3" 或 "3/12"（轨号/总数）
	CueSourcePath       string  `json:"cue_source_path,omitempty"`                         // CUE 来源路径（非空表示 CUE 拆分歌曲）
	CueTrackIndex       int     `json:"cue_track_index,omitempty"`                         // CUE track 序号 (1-99)
	CueAudioPath        string  `json:"-"`                                                 // 原始整轨音频路径（内部使用）
	CueStartSeconds     float64 `json:"-"`                                                 // CUE track 在源音频中的起始秒数
	CueEndSeconds       float64 `json:"-"`                                                 // CUE track 在源音频中的结束秒数（0 表示到文件末尾）

	AddedAt         time.Time  `json:"added_at" example:"2024-01-01T12:00:00Z"`   // 添加时间
	UpdatedAt       time.Time  `json:"updated_at" example:"2024-01-01T12:00:00Z"` // 最后更新时间
	FileModifiedAt  *time.Time `json:"file_modified_at,omitempty"`                // 文件修改时间(mtime，本地歌曲扫描时记录；未知为 nil)
	PlaylistAddedAt *time.Time `json:"playlist_added_at,omitempty"`               // 加入当前歌单的时间，仅歌单成员查询返回
}

// SongBrief is a lightweight song representation for list display.
type SongBrief struct {
	ID              int64      `json:"id"`
	Title           string     `json:"title"`
	Artist          string     `json:"artist"`
	Album           string     `json:"album"`
	Duration        float64    `json:"duration"`
	CoverURL        string     `json:"cover_url"`
	PlaylistAddedAt *time.Time `json:"playlist_added_at,omitempty"`
}

// Brief returns a lightweight representation for list display.
func (s *Song) Brief() SongBrief {
	return SongBrief{
		ID:              s.ID,
		Title:           s.Title,
		Artist:          s.Artist,
		Album:           s.Album,
		Duration:        s.Duration,
		CoverURL:        s.CoverURLPath(),
		PlaylistAddedAt: s.PlaylistAddedAt,
	}
}

// IsPluginSourced 判断是否插件来源的歌曲
func (s *Song) IsPluginSourced() bool {
	return s.PluginEntryPath != "" && s.SourceData != ""
}

// IsHLSStream 判断歌曲源头是否为 HLS 播放列表（.m3u8/.m3u）。
// 仅电台类型且源 URL 后缀匹配时返回 true。
func (s *Song) IsHLSStream() bool {
	if s.Type != TypeRadio || s.URL == "" {
		return false
	}
	u, err := url.Parse(s.URL)
	if err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(u.Path))
	return ext == ".m3u8" || ext == ".m3u"
}

// PlaybackURL 返回客户端用的统一播放 URL(所有 type 都用同一形态)。
// 客户端只需要 setUrl(song.url),不需要判断 type。
// handler 内部按 type 分发到本地 ServeFile / Orchestrator / 直链下载 / 电台 302。
//
// HLS 电台特殊处理:URL 末尾追加 .m3u8 后缀。
// 原因:ExoPlayer (Android) / AVPlayer (iOS) / 部分桌面 player 在 setMediaSource 阶段
// 根据 URL 后缀决定 MediaSourceType。无后缀的 .../play 会被推断为 ProgressiveMediaSource
// (普通 MP3/AAC),即使后端 302 跳到 .m3u8 或直接返回 m3u8 文本,player 也不会切换到
// HlsMediaSource,导致按 Progressive 协议解析 m3u8 文本失败 → 整首电台无法播放。
func (s *Song) PlaybackURL() string {
	if s.ID == 0 {
		return ""
	}
	if s.IsHLSStream() {
		return fmt.Sprintf("/api/v1/songs/%d/play.m3u8", s.ID)
	}
	return fmt.Sprintf("/api/v1/songs/%d/play", s.ID)
}

// CoverURLPath 返回客户端用的统一封面 URL。
// 有封面(本地或远程)时返回 /api/v1/songs/{id}/cover 端点,后端自动判断本地/远程
// 无封面时返回空字符串,避免客户端发起注定 404 的请求。
// 当存在封面提供者插件时,本地无封面歌曲也放行,让客户端请求触发自动搜索。
func (s *Song) CoverURLPath() string {
	if s.ID == 0 {
		return ""
	}
	if s.CoverPath != "" || s.CoverURL != "" {
		return fmt.Sprintf("/api/v1/songs/%d/cover?v=%d", s.ID, s.UpdatedAt.Unix())
	}
	if s.Type == TypeLocal && GetHasCoverProvider() != nil && GetHasCoverProvider()() {
		return fmt.Sprintf("/api/v1/songs/%d/cover?v=%d", s.ID, s.UpdatedAt.Unix())
	}
	return ""
}

// hasLyricProvider 由上层(app 初始化时)注入，报告当前是否存在已启用的歌词提供者插件。
// LyricURLPath 用它决定是否对本地无歌词歌曲放行歌词 URL —— 有歌词插件时才放行，
// 避免没装插件的用户对全库无歌词歌发出注定 404 的请求。未注入(nil)时视作“无插件”。
var hasLyricProvider atomic.Value // stores func() bool

// hasCoverProvider 由上层(app 初始化时)注入，报告当前是否存在已启用的封面提供者插件。
// CoverURLPath 用它决定是否对本地无封面歌曲放行封面 URL —— 有封面插件时才放行，
// 避免没装插件的用户对全库无封面歌发出注定 404 的请求。未注入(nil)时视作“无插件”。
var hasCoverProvider atomic.Value // stores func() bool

// SetHasLyricProvider 注入歌词提供者检查函数（启动时调用）。
func SetHasLyricProvider(fn func() bool) {
	hasLyricProvider.Store(fn)
}

// GetHasLyricProvider 返回当前歌词提供者检查函数，未注入时返回 nil。
func GetHasLyricProvider() func() bool {
	if v := hasLyricProvider.Load(); v != nil {
		return v.(func() bool)
	}
	return nil
}

// SetHasCoverProvider 注入封面提供者检查函数（启动时调用）。
func SetHasCoverProvider(fn func() bool) {
	hasCoverProvider.Store(fn)
}

// GetHasCoverProvider 返回当前封面提供者检查函数，未注入时返回 nil。
func GetHasCoverProvider() func() bool {
	if v := hasCoverProvider.Load(); v != nil {
		return v.(func() bool)
	}
	return nil
}

// LyricURLPath 返回客户端用的统一歌词 URL。
// 有歌词时(无论来源):返回 /api/v1/songs/{id}/lyric 端点。
// remote 歌曲、以及"无歌词但存在歌词插件"的本地歌曲即使暂无歌词也返回端点,
// 使客户端能发起请求触发歌词插件自动搜索。其他情况无歌词时:返回空字符串。
func (s *Song) LyricURLPath() string {
	if s.ID == 0 {
		return ""
	}
	// Lyric 字段(本地落库的 JSON payload)非空,或 lyric_source 仍是 url 且 LyricRemoteURL 非空,
	// 才视作有歌词。后者保持 LyricRemoteURL 与 GetSongLyric 实际分发(按 lyric_source)语义一致 ——
	// 远程歌转 local 时 lyric_remote_url 会被保留作档案,但 lyric_source 已经不再是 "url",
	// 此时不应再误报"有歌词"导致前端发出注定 404 的请求。
	//
	// remote 歌曲额外放行：插件添加的远程歌曲初始通常无歌词,但 GetSongLyric handler
	// 在 payload 为空时会 fallback 到已注册的歌词搜索插件。若此处不返回 URL,客户端
	// 永远不会发起歌词请求,搜索逻辑无法被激活。(#201)
	if s.Lyric != "" || (s.LyricSource == LyricSourceURL && s.LyricRemoteURL != "") || s.Type == TypeRemote {
		return fmt.Sprintf("/api/v1/songs/%d/lyric", s.ID)
	}
	// 本地无歌词歌曲：仅当存在已启用的歌词提供者插件时才放行,让客户端发起请求触发自动搜索。
	// 否则(没装/没启用歌词插件)保持返回空,行为与历史一致 —— 不平白让客户端对全库发 404 请求。
	// 限定 TypeLocal：radio 是直播流、不参与歌词搜索,不应因存在歌词插件被放行。(#303)
	if s.Type == TypeLocal && GetHasLyricProvider() != nil && GetHasLyricProvider()() {
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
	cp := *s // 值拷贝，不修改原始 *Song
	if cp.ID != 0 {
		if cp.Type != TypeLocal && cp.URL != "" {
			cp.SourceURL = cp.URL
		}
		if cp.CoverURL != "" {
			cp.SourceCoverURL = cp.CoverURL
		}
		cp.URL = cp.PlaybackURL()
		cp.CoverURL = cp.CoverURLPath()
		cp.LyricURL = cp.LyricURLPath()
	}
	return json.Marshal((*songAlias)(&cp))
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
		// 插件音源歌曲(plugin_entry_path + source_data)靠 source_data 播放，URL 合法为空；
		// 其余 remote/radio 必须有直链 URL。
		if s.URL == "" && !s.IsPluginSourced() {
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
// 无独立封面但有成员时仍返回端点，由端点按需回退到成员封面。
//
// URL 携带 ?v=UpdatedAt 版本号:歌单封面端点缓存头是长 max-age,而 URL 按 ID 固定,
// 用户替换封面后若 URL 不变,浏览器会一直用旧缓存(表现为封面上传后普通刷新/页面内
// 导航仍显示旧图)。上传封面走 PlaylistService.UploadCover → Update,必然刷新
// updated_at,故 ?v= 会随之变化,天然穿透缓存。与歌曲封面 Song.CoverURLPath 一致。
func (p *Playlist) CoverURLPath() string {
	if p.ID == 0 {
		return ""
	}
	if p.CoverPath == "" && p.CoverURL == "" && p.SongCount == 0 {
		return ""
	}
	return fmt.Sprintf("/api/v1/playlists/%d/cover?v=%d", p.ID, p.UpdatedAt.Unix())
}

// MarshalJSON 序列化时把 CoverURL 字段统一覆盖为服务端端点。
// 这样所有歌单接口返回的 cover_url 都是 /api/v1/playlists/{id}/cover 或原始 CDN URL。
// 客户端拿到后直接使用,无需关心 cover_path 等内部细节。
func (p *Playlist) MarshalJSON() ([]byte, error) {
	type playlistAlias Playlist
	cp := *p // 值拷贝，不修改原始 *Playlist
	if cp.ID != 0 {
		cp.CoverURL = cp.CoverURLPath()
	}
	return json.Marshal((*playlistAlias)(&cp))
}

// Playlist 歌单结构体
type Playlist struct {
	ID          int64      `json:"id" example:"1"`                                       // 歌单 ID
	Type        string     `json:"type" example:"normal" enums:"normal,radio"`           // 歌单类型：normal/radio
	Name        string     `json:"name" example:"我的最爱"`                                  // 歌单名称
	Description string     `json:"description" example:"收藏的经典歌曲"`                        // 歌单描述
	CoverPath   string     `json:"-"`                                                    // 封面图片本地路径(内部使用,不暴露给客户端)
	CoverURL    string     `json:"cover_url" example:"https://example.com/playlist.jpg"` // 封面图片 URL
	Labels      []string   `json:"labels" example:"[\"built_in\"]"`                      // 歌单标签，如 ["built_in"]
	SortBy      string     `json:"sort_by" example:"position"`                           // 视图排序字段：position/added_at/file_modified_at/title/artist/duration
	SortOrder   string     `json:"sort_order" example:"asc"`                             // 视图排序方向：asc/desc
	SongCount   int        `json:"song_count" example:"10"`                              // 歌曲数量
	RemoteCount int        `json:"remote_count" example:"3"`                             // 歌单内网络歌曲（songs.type=remote）数量，>0 即为「网络歌单」；仅列表接口填充，详情接口恒为 0
	PinnedAt    *time.Time `json:"pinned_at,omitempty" example:"2024-01-01T12:00:00Z"`   // 置顶时间，nil 表示未置顶；多个置顶歌单按此字段倒序排列
	CreatedAt   time.Time  `json:"created_at" example:"2024-01-01T12:00:00Z"`            // 创建时间
	UpdatedAt   time.Time  `json:"updated_at" example:"2024-01-01T12:00:00Z"`            // 最后更新时间
}

// 播放上下文的类型标识。合法取值为「歌单」「自定义标签」或歌曲分面维度
// （artist/album/genre/year/decade/language/style），分面维度由
// database.IsSongFacetField 判定 —— 校验入口见 services.IsValidPlayContextType。
const (
	PlayContextPlaylist = "playlist" // 歌单（key 为歌单 ID）
	PlayContextTag      = "tag"      // 自定义标签分类（key 为 song-tag ID）
)

// PlayHistoryEntry 某播放上下文内的一条单曲播放历史。
type PlayHistoryEntry struct {
	Song      *Song     `json:"song"`                                     // 歌曲详情
	PlayedAt  time.Time `json:"played_at" example:"2024-01-01T12:00:00Z"` // 最后一次播放时间
	PlayCount int       `json:"play_count" example:"3"`                   // 在该上下文内的累计播放次数
}

// PlayHistoryListResponse 播放历史查询响应。
type PlayHistoryListResponse struct {
	Items []PlayHistoryEntry `json:"items"`              // 播放记录，按最后播放时间倒序
	Total int                `json:"total" example:"12"` // 本次返回的条数（上限 50，故与 items 长度一致）
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

// IsBuiltIn 判断是否为内置歌单
func (p *Playlist) IsBuiltIn() bool {
	return p.HasLabel(PlaylistLabelBuiltIn)
}

// IsPinned 判断歌单是否已置顶
func (p *Playlist) IsPinned() bool {
	return p.PinnedAt != nil
}

// HasLabel 判断歌单是否包含指定标签
func (p *Playlist) HasLabel(label string) bool {
	for _, l := range p.Labels {
		if l == label {
			return true
		}
	}
	return false
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
	Version           string `json:"version" example:"v1.0.0"`                                                                                 // 版本号
	GitCommit         string `json:"git_commit" example:"abc123"`                                                                              // Git 提交哈希
	BuildTime         string `json:"build_time" example:"2024-01-01_12:00:00"`                                                                 // 构建时间
	DownloadURLPrefix string `json:"download_url_prefix" example:"https://github.com/songloft-org/songloft/releases/download/v1.0.0/songloft"` // 下载地址前缀
	ReleaseNotes      string `json:"release_notes" example:"正式版本更新说明"`                                                                         // 发布说明
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
	UpgradeStatusUploading   = "uploading"   // 上传中
	UpgradeStatusDownloading = "downloading" // 下载中
	UpgradeStatusTesting     = "testing"     // 测试中
	UpgradeStatusReplacing   = "replacing"   // 替换中
	UpgradeStatusResetting   = "resetting"   // 回退中
	UpgradeStatusRestarting  = "restarting"  // 重启中
	UpgradeStatusFailed      = "failed"      // 失败
)

// UploadedBinaryInfo 上传升级时返回的二进制文件信息
type UploadedBinaryInfo struct {
	Version         string `json:"version" example:"v2.11.0"`         // 上传文件的版本号
	Channel         string `json:"channel" example:"stable"`          // 上传文件的通道（stable/dev）
	BuildTime       string `json:"build_time" example:"2026-08-01"`   // 上传文件的构建时间
	BuildType       string `json:"build_type" example:"full"`         // 上传文件的构建类型（full/lite）
	CurrentVersion  string `json:"current_version" example:"v2.10.0"` // 当前运行版本
	CurrentChannel  string `json:"current_channel" example:"dev"`     // 当前运行通道
	ChannelMismatch bool   `json:"channel_mismatch" example:"true"`   // 通道是否不匹配（需前端二次确认）
}

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
	PlaylistLabelHidden      = "hidden"       // 隐藏歌单标签

	PlaylistModeDirectory = "directory" // 每个文件夹生成独立歌单
	PlaylistModeTopLevel  = "top_level" // 按一级子目录合并歌单
	PlaylistModeBubbleUp  = "bubble_up" // 歌曲同时出现在所有上级文件夹歌单
)

// BatchDeleteSongsRequest 批量删除歌曲请求
type BatchDeleteSongsRequest struct {
	IDs         []int64 `json:"ids" example:"1"`              // 要删除的歌曲 ID 列表
	DeleteFiles bool    `json:"delete_files" example:"false"` // 是否同步删除本地音频文件
}

// BatchDeleteSongsResponse 批量删除歌曲响应
type BatchDeleteSongsResponse struct {
	Deleted int `json:"deleted" example:"3"` // 实际删除的数量
}

// BatchDeletePlaylistsRequest 批量删除歌单请求
type BatchDeletePlaylistsRequest struct {
	IDs         []int64 `json:"ids" example:"1"`              // 要删除的歌单 ID 列表
	DeleteSongs bool    `json:"delete_songs" example:"false"` // 为 true 时，一并删除仅属于被删歌单的孤儿歌曲（含本地歌曲及其磁盘文件）
}

// BatchDeletePlaylistsResponse 批量删除歌单响应
type BatchDeletePlaylistsResponse struct {
	Deleted      int `json:"deleted" example:"3"`        // 实际删除的歌单数量
	DeletedSongs int `json:"deleted_songs" example:"12"` // 连带清理的孤儿歌曲数量
}

// UpdatePlaylistRequest 更新歌单请求
type UpdatePlaylistRequest struct {
	Audiobook   *bool   `json:"audiobook,omitempty"`
	Name        string  `json:"name" example:"我的最爱"`                  // 歌单名称
	Description string  `json:"description" example:"收藏的经典歌曲"`        // 歌单描述
	CoverPath   *string `json:"cover_path,omitempty" example:""`      // 封面图片本地路径（传空字符串清除）
	CoverURL    *string `json:"cover_url,omitempty" example:""`       // 封面图片 URL（传空字符串清除）
	CoverSongID *int64  `json:"cover_song_id,omitempty" example:"42"` // 从指定歌曲复制封面（与 cover_path/cover_url 互斥，优先级更高）
}

// SetPlaylistVisibilityRequest 设置歌单可见性请求
type SetPlaylistVisibilityRequest struct {
	Hidden bool `json:"hidden" example:"true"` // 是否隐藏歌单
}

// SetPlaylistPinnedRequest 设置歌单置顶状态请求
type SetPlaylistPinnedRequest struct {
	Pinned bool `json:"pinned" example:"true"` // 是否置顶歌单
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
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	Author         string   `json:"author"`
	Homepage       string   `json:"homepage,omitempty"`
	License        string   `json:"license,omitempty"`
	EntryPath      string   `json:"entry_path"` // 路由前缀（如 "myplugin"）
	Main           string   `json:"main"`       // 入口文件路径（如 "main.js"）
	MinHostVersion string   `json:"min_host_version,omitempty"`
	Permissions    []string `json:"permissions"`    // 权限列表
	PublicPaths    []string `json:"public_paths"`   // 无需 JWT 认证的路径前缀
	ExternalPaths  []string `json:"external_paths"` // 可访问的外部绝对路径目录
	Icon           string   `json:"icon,omitempty"`
	UpdateURL      string   `json:"update_url,omitempty"`
	DownloadURL    string   `json:"download_url,omitempty"`
	// RenderEngine 插件页渲染引擎："webview" / "lynx"；空串 = 跟随宿主默认（当前 webview）。
	// 刻意不加 omitempty：客户端需要能稳定读到该字段并自行把空串映射为默认引擎。
	RenderEngine string         `json:"render_engine"`
	Status       JSPluginStatus `json:"status"`
	ZipHash      string         `json:"zip_hash,omitempty"`   // ZIP 文件 SHA256
	EntryHash    string         `json:"entry_hash,omitempty"` // main.js/main.jsc 内容 SHA256
	FileModTime  string         `json:"file_mod_time,omitempty"`
	FilePath     string         `json:"file_path"` // ZIP 文件相对路径
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}
