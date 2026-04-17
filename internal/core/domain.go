package core

import (
	"regexp"
	"strings"
)

// domainRegex: basic validation - labels separated by dots, valid chars, TLD >= 2 chars
var domainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

// NormalizeDomain cleans a raw string into canonical form, or returns "" if invalid.
func NormalizeDomain(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)

	s = strings.TrimLeft(s, "-*•·‣▪◦ \t")
	for i, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '.' || r == ')' {
			if i > 0 && i < len(s)-1 {
				s = strings.TrimSpace(s[i+1:])
			}
		}
		break
	}

	s = strings.Trim(s, `"'`+"`"+`*_~()[]{}<>`)

	for _, proto := range []string{"https://", "http://", "ftp://"} {
		s = strings.TrimPrefix(s, proto)
	}

	for _, sep := range []string{"/", " ", ",", "?", "#", "\t"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			s = s[:idx]
		}
	}

	s = strings.TrimRight(s, ".,;:)")
	s = strings.TrimPrefix(s, "www.")
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		s = s[:idx]
	}

	if s == "" {
		return ""
	}
	if !domainRegex.MatchString(s) {
		return ""
	}

	labels := strings.Split(s, ".")
	tld := labels[len(labels)-1]
	if isAllDigits(tld) {
		return ""
	}
	return s
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// MatchesAnyTLD returns true if domain ends with any of the given TLD suffixes.
// TLDs should already be lowercase. Empty list means "match all".
func MatchesAnyTLD(domain string, tlds []string) bool {
	if len(tlds) == 0 {
		return true
	}
	for _, t := range tlds {
		t = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(t)), ".")
		if t == "" {
			continue
		}
		if strings.HasSuffix(domain, "."+t) || domain == t {
			return true
		}
	}
	return false
}

// ExtractDomains parses AI response text and pulls out everything that
// normalizes to a valid domain.
func ExtractDomains(text string) []string {
	seen := make(map[string]struct{})
	var out []string

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if d := NormalizeDomain(line); d != "" {
			if _, ok := seen[d]; !ok {
				seen[d] = struct{}{}
				out = append(out, d)
			}
			continue
		}

		fields := strings.FieldsFunc(line, func(r rune) bool {
			return r == ' ' || r == '\t' || r == ',' || r == ';' || r == '|' ||
				r == '*' || r == '_' || r == '(' || r == ')' || r == '[' || r == ']'
		})
		for _, f := range fields {
			if d := NormalizeDomain(f); d != "" {
				if _, ok := seen[d]; !ok {
					seen[d] = struct{}{}
					out = append(out, d)
				}
			}
		}
	}
	return out
}