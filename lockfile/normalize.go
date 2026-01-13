package lockfile

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	darwinPlatformVersionRegex = regexp.MustCompile(`^(.*-darwin)(\d+)$`)
	linuxLibcRegex             = regexp.MustCompile(`^(.*-linux)-(gnu|musl)$`)
)

func normalizePlatformForLockfileOutput(platform string) string {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	if normalized == "" {
		return ""
	}

	if matches := darwinPlatformVersionRegex.FindStringSubmatch(normalized); matches != nil {
		normalized = matches[1] + "-" + matches[2]
	}

	if matches := linuxLibcRegex.FindStringSubmatch(normalized); matches != nil {
		normalized = matches[1]
	}

	return normalized
}

func normalizePathRemoteForLockfileOutput(remote string) string {
	if remote == "" {
		return remote
	}
	if filepath.IsAbs(remote) {
		return remote
	}
	cleaned := filepath.Clean(remote)
	prefix := "." + string(filepath.Separator)
	if strings.HasPrefix(cleaned, prefix) {
		return strings.TrimPrefix(cleaned, prefix)
	}
	return cleaned
}

type constraintPart struct {
	raw    string
	weight int
	index  int
	verStr string
}

//nolint:gocyclo // Bundler-style ordering requires a few rule checks.
func normalizeConstraintsForLockfile(constraints []string) []string {
	clean := cleanConstraints(constraints)
	if len(clean) == 0 {
		return nil
	}

	for _, c := range clean {
		if strings.Contains(c, "||") {
			return clean
		}
	}

	parts := make([]constraintPart, 0, len(clean))
	index := 0
	for _, constraint := range clean {
		expanded := strings.ReplaceAll(constraint, "&", ",")
		for _, part := range strings.Split(expanded, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			weight, ok := constraintWeight(part)
			if !ok {
				return clean
			}
			verStr := ""
			if weight == 4 {
				fields := strings.Fields(part)
				if len(fields) >= 2 {
					verStr = fields[len(fields)-1]
				}
			}
			parts = append(parts, constraintPart{
				raw:    part,
				weight: weight,
				index:  index,
				verStr: verStr,
			})
			index++
		}
	}

	if len(parts) < 2 {
		if len(parts) == 1 {
			return []string{parts[0].raw}
		}
		return nil
	}

	slices.SortStableFunc(parts, func(a, b constraintPart) int {
		if a.weight == b.weight {
			if a.weight == 4 && a.verStr != "" && b.verStr != "" {
				// For "!=" constraints, Bundler orders by version string descending.
				return strings.Compare(b.verStr, a.verStr)
			}
			if a.index == b.index {
				return 0
			}
			if a.index < b.index {
				return -1
			}
			return 1
		}
		if a.weight < b.weight {
			return -1
		}
		return 1
	})

	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, part.raw)
	}
	return out
}

func cleanConstraints(constraints []string) []string {
	out := make([]string, 0, len(constraints))
	for _, c := range constraints {
		c = strings.TrimSpace(c)
		if c == "" || c == "*" || c == ">= 0" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func constraintWeight(part string) (int, bool) {
	switch {
	case strings.HasPrefix(part, "~>"):
		return 0, true
	case strings.HasPrefix(part, ">="), strings.HasPrefix(part, ">"):
		return 1, true
	case strings.HasPrefix(part, "=="), strings.HasPrefix(part, "="):
		return 2, true
	case strings.HasPrefix(part, "<="), strings.HasPrefix(part, "<"):
		return 3, true
	case strings.HasPrefix(part, "!="):
		return 4, true
	default:
		return 0, false
	}
}
