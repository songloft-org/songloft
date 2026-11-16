package database

import (
	"database/sql/driver"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("songloft_tag_number", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			value, _ := args[0].(string)
			return tagNumber(value), nil
		})
}

// tagNumber reads the positive number in tags such as "02/12". Invalid or
// missing values return NULL so sorting can give them an explicit fallback.
func tagNumber(value string) driver.Value {
	number, _, _ := strings.Cut(strings.TrimSpace(value), "/")
	number = strings.TrimSpace(number)
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return nil
		}
	}
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	return n
}

// applySongOrder runs before pagination for both complete songs and ID lists.
// An album without an explicit sort uses disc/track order; other views keep
// their existing default. Unnumbered discs belong to disc 1, and unnumbered
// tracks follow numbered tracks within their disc.
func applySongOrder(sb sq.SelectBuilder, filter *SongFilter) sq.SelectBuilder {
	if filter.OrderBy != "track" && !(filter.OrderBy == "" && filter.Album != "") {
		return applyOrder(sb, filter.OrderBy, filter.Order, "added_at DESC", songOrderWhitelist, "")
	}
	dir := "ASC"
	if strings.EqualFold(filter.Order, "DESC") {
		dir = "DESC"
	}
	track := "COALESCE(songloft_tag_number(track), NULLIF(cue_track_index, 0))"
	return sb.OrderBy(
		"COALESCE(songloft_tag_number(disc), 1) "+dir,
		track+" IS NULL ASC",
		track+" "+dir,
		"file_path ASC", "id ASC",
	)
}
