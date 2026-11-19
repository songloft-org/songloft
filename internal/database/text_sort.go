package database

import (
	"strings"

	"github.com/mozillazg/go-pinyin"
)

func nullableTextSortKey(value any) any {
	if text, ok := value.(string); ok {
		return textSortKey(text)
	}
	return nil
}

// textSortKey interleaves Chinese pinyin with Latin titles, ignoring case and
// surrounding whitespace. Use the dictionary's default reading for polyphones.
// Keep digits, punctuation and unsupported scripts rather than dropping them.
func textSortKey(text string) string {
	args := pinyin.NewArgs()
	args.Fallback = func(r rune, _ pinyin.Args) []string { return []string{string(r)} }
	return strings.Join(pinyin.LazyPinyin(strings.ToLower(strings.TrimSpace(text)), args), "")
}
