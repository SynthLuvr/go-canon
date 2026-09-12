// Package cover parses `go tool cover -func` output for the coverage gate.
package cover

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ParseTotal extracts the total statement coverage percentage from the
// output of `go tool cover -func`.
func ParseTotal(out string) (float64, error) {
	for line := range strings.Lines(out) {
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		// The "total:" prefix guarantees at least one field.
		fields := strings.Fields(line)
		last := fields[len(fields)-1]
		pct, err := strconv.ParseFloat(strings.TrimSuffix(last, "%"), 64)
		if err != nil {
			return 0, fmt.Errorf("parse total coverage %q: %w", last, err)
		}
		return pct, nil
	}
	return 0, errors.New("no total line in cover output")
}
