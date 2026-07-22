package services

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"songloft/internal/models"
)

const defaultPathTemplate = "downloads/{artist}-{album}/{title}"

var knownPlaceholders = map[string]bool{
	"title":  true,
	"artist": true,
	"album":  true,
	"year":   true,
	"genre":  true,
}

type templateSegment struct {
	literal     string
	placeholder string // "title"/"artist"/... or "" for pure literal
}

type PathTemplate struct {
	raw      string
	segments []templateSegment
}

func ParsePathTemplate(template string) (*PathTemplate, error) {
	if template == "" {
		return nil, fmt.Errorf("模板不能为空")
	}
	if strings.Contains(template, "..") {
		return nil, fmt.Errorf("模板不允许包含 '..'")
	}
	if strings.Contains(template, "//") {
		return nil, fmt.Errorf("模板不允许包含连续 '/'")
	}
	if strings.HasPrefix(template, "/") {
		return nil, fmt.Errorf("模板不允许以 '/' 开头")
	}

	var segments []templateSegment
	hasTitle := false
	rest := template

	for len(rest) > 0 {
		idx := strings.Index(rest, "{")
		if idx < 0 {
			segments = append(segments, templateSegment{literal: rest})
			break
		}
		if idx > 0 {
			segments = append(segments, templateSegment{literal: rest[:idx]})
		}
		end := strings.Index(rest[idx:], "}")
		if end < 0 {
			return nil, fmt.Errorf("未闭合的占位符 '{'")
		}
		name := rest[idx+1 : idx+end]
		if !knownPlaceholders[name] {
			return nil, fmt.Errorf("未知占位符 {%s}，支持: title, artist, album, year, genre", name)
		}
		if name == "title" {
			hasTitle = true
		}
		segments = append(segments, templateSegment{placeholder: name})
		rest = rest[idx+end+1:]
	}

	if !hasTitle {
		return nil, fmt.Errorf("模板必须包含 {title} 占位符")
	}

	return &PathTemplate{raw: template, segments: segments}, nil
}

func (t *PathTemplate) Render(song *models.Song) string {
	var b strings.Builder
	for _, seg := range t.segments {
		if seg.placeholder == "" {
			b.WriteString(seg.literal)
			continue
		}
		val := placeholderValue(seg.placeholder, song)
		// 占位符取值内部的 '/' 不是模板里的目录分隔符，先折叠成 '_'，
		// 否则含斜杠的歌名（如 "AC/DC"）会被下面的 Split 误当成目录层级，
		// 导致 '/' 前的文字变成文件夹名（issue #265）。
		val = strings.ReplaceAll(val, "/", "_")
		b.WriteString(val)
	}

	parts := strings.Split(b.String(), "/")
	for i, part := range parts {
		parts[i] = sanitizePathComponent(part)
	}

	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return "Unknown"
	}
	return strings.Join(result, "/")
}

func (t *PathTemplate) String() string {
	return t.raw
}

func placeholderValue(name string, song *models.Song) string {
	if song == nil {
		return defaultForPlaceholder(name)
	}
	switch name {
	case "title":
		if song.Title != "" {
			return song.Title
		}
		return "Unknown"
	case "artist":
		if song.Artist != "" {
			return song.Artist
		}
		return "Unknown Artist"
	case "album":
		if song.Album != "" {
			return song.Album
		}
		return "Unknown Album"
	case "year":
		if song.Year > 0 {
			return strconv.Itoa(song.Year)
		}
		return ""
	case "genre":
		if song.Genre != "" {
			return song.Genre
		}
		return ""
	}
	return ""
}

func defaultForPlaceholder(name string) string {
	switch name {
	case "title":
		return "Unknown"
	case "artist":
		return "Unknown Artist"
	case "album":
		return "Unknown Album"
	default:
		return ""
	}
}

const maxComponentLen = 200

func sanitizePathComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if isUnsafeChar(r) {
			if !prevSpace {
				b.WriteRune('_')
				prevSpace = true
			}
			continue
		}
		if r == ' ' {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}

	result := strings.TrimSpace(b.String())
	result = strings.TrimRight(result, ".-_ ")

	if len(result) > maxComponentLen {
		result = result[:maxComponentLen]
	}
	return result
}

func isUnsafeChar(r rune) bool {
	switch r {
	case '\\', ':', '*', '?', '"', '<', '>', '|':
		return true
	}
	return unicode.IsControl(r)
}
