package store

import (
	"fmt"
	"strings"
)

// splitFilters splits a comma-separated filter string into a slice.
// Returns nil for empty strings.
func splitFilters(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// scanFilters implements sql.Scanner to deserialize a comma-separated string
// into a []string slice during row scanning.
type scanFilters struct {
	dest *[]string
}

func (sf *scanFilters) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*sf.dest = splitFilters(v)
	case []byte:
		*sf.dest = splitFilters(string(v))
	case nil:
		*sf.dest = nil
	default:
		return fmt.Errorf("scanFilters: unsupported type %T", src)
	}
	return nil
}
