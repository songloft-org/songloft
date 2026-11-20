package database

import (
	"fmt"
	"strings"

	"songloft/internal/models"
)

// ParseSongTypes parses the public songs type query as a comma-separated set.
// An empty value means that no type filter was requested. The returned values
// are deduplicated while preserving their first-seen order.
func ParseSongTypes(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	valid := map[string]struct{}{
		models.TypeLocal:  {},
		models.TypeRemote: {},
		models.TypeRadio:  {},
	}
	seen := make(map[string]struct{}, len(valid))
	types := make([]string, 0, len(valid))
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := valid[value]; !ok {
			return nil, fmt.Errorf("unsupported song type %q", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		types = append(types, value)
	}
	return types, nil
}
