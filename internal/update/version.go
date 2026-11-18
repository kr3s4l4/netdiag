package update

import (
	"strconv"
	"strings"
)

// CompareVersions compares two SemVer-like versions (v1.2.3, 1.2.3-rc.1).
// It returns >0 when a is newer, <0 when b is newer and 0 when they are
// equivalent. Development builds are treated as 0.0.0 so any signed release is
// considered newer.
func CompareVersions(a, b string) int {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa.num[i] > pb.num[i] {
			return 1
		}
		if pa.num[i] < pb.num[i] {
			return -1
		}
	}
	return comparePrerelease(pa.pre, pb.pre)
}

type parsedVersion struct {
	num [3]int
	pre string
}

func parseVersion(raw string) parsedVersion {
	var out parsedVersion
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		if s[i] == '-' {
			rest := s[i+1:]
			if j := strings.IndexByte(rest, '+'); j >= 0 {
				rest = rest[:j]
			}
			out.pre = rest
		}
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil && n >= 0 {
			out.num[i] = n
		}
	}
	return out
}

func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1 // stable > prerelease
	}
	if b == "" {
		return -1
	}
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
		case aerr == nil:
			return -1 // numeric identifiers sort before alphanumeric
		case berr == nil:
			return 1
		default:
			if as[i] != bs[i] {
				if as[i] > bs[i] {
					return 1
				}
				return -1
			}
		}
	}
	if len(as) > len(bs) {
		return 1
	}
	if len(as) < len(bs) {
		return -1
	}
	return 0
}

// IsNewer reports whether candidate is a valid, strictly newer version than
// current. Empty and development versions are ignored as candidates.
func IsNewer(candidate, current string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || strings.EqualFold(candidate, "dev") {
		return false
	}
	pa := parseVersion(candidate)
	if pa.num == [3]int{} {
		return false
	}
	return CompareVersions(candidate, current) > 0
}
