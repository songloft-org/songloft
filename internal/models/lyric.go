package models

import (
	"encoding/json"
	"strings"
)

// LyricPayload 是 songs.lyric 字段在数据库中的存储格式,也是 /api/v1/songs/{id}/lyric
// API 响应 data 字段的形态。
//
// 设计目的:让本地歌词(只有主歌词)与插件歌词(可能含翻译/罗马音/逐字)用同一种载体表达,
// 前后端契约统一。所有字段都是 LRC 文本。
type LyricPayload struct {
	Lyric   string `json:"lyric"`             // 主歌词
	Tlyric  string `json:"tlyric,omitempty"`  // 翻译歌词
	Rlyric  string `json:"rlyric,omitempty"`  // 罗马音歌词
	Lxlyric string `json:"lxlyric,omitempty"` // 逐字歌词
}

// IsEmpty 表示 payload 不含任何歌词文本。
func (p LyricPayload) IsEmpty() bool {
	return p.Lyric == "" && p.Tlyric == "" && p.Rlyric == "" && p.Lxlyric == ""
}

// MarshalString 把 LyricPayload 序列化为字符串入库。空 payload 返回空字符串
// (而非 "{}"),便于 SQL 层 lyric != ” 这类判空保持原语义。
func (p LyricPayload) MarshalString() string {
	if p.IsEmpty() {
		return ""
	}
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// UnmarshalLyric 把数据库 lyric 列还原成 LyricPayload。
//
// 兼容 3 种历史/异常形态:
//  1. 空字符串 -> 空 payload
//  2. 合法 JSON({"lyric":"...","tlyric":"..."}) -> 直接解析
//  3. 裸 LRC 文本(从未迁移过的旧数据)-> 整段塞进 Lyric 字段
func UnmarshalLyric(raw string) LyricPayload {
	if raw == "" {
		return LyricPayload{}
	}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") {
		var p LyricPayload
		if err := json.Unmarshal([]byte(trimmed), &p); err == nil {
			return p
		}
	}
	return LyricPayload{Lyric: raw}
}

// LyricPayloadFromLRC 用单段 LRC 文本构造 payload,用于 tag/lrc 等只有主歌词的写入场景。
func LyricPayloadFromLRC(text string) LyricPayload {
	return LyricPayload{Lyric: text}
}

// ApplyLyricToSong 把"语义化"的 lyric + source 转成 Song 字段的存储形态。
//
//   - lyricSource == LyricSourceURL: text 视作待拉取的 URL,落到 LyricRemoteURL,Lyric 清空
//   - 其它来源(file/embedded/scraped/cached/空): text 视作 LRC 文本,
//     包装成 LyricPayload JSON 落到 Lyric,LyricRemoteURL 清空
//
// 调用方在构造/更新 Song 时统一走这个入口,写库路径就不必关心存储形态。
func ApplyLyricToSong(s *Song, text, lyricSource string) {
	s.LyricSource = lyricSource
	if lyricSource == LyricSourceURL {
		s.Lyric = ""
		s.LyricRemoteURL = text
		return
	}
	s.Lyric = LyricPayloadFromLRC(text).MarshalString()
	s.LyricRemoteURL = ""
}
