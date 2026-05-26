package service

import (
	"fmt"
	"sort"
	"strings"
)

var allowedScopes = map[string]struct{}{
	"admin":          {},
	"auth:write":     {},
	"full":           {},
	"ledger:read":    {},
	"ledger:write":   {},
	"payments:read":  {},
	"payments:write": {},
	"webhooks:read":  {},
	"webhooks:write": {},
}

func NormalizeScope(scope string) (string, error) {
	items := splitScopes(scope)
	if len(items) == 0 {
		return "full", nil
	}

	seen := make(map[string]struct{}, len(items))
	normalized := make([]string, 0, len(items))

	for _, item := range items {
		if _, ok := allowedScopes[item]; !ok {
			return "", fmt.Errorf("invalid scope %q", item)
		}
		if item == "full" {
			return "full", nil
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	sort.Strings(normalized)
	return strings.Join(normalized, " "), nil
}

func splitScopes(scope string) []string {
	parts := strings.FieldsFunc(scope, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(strings.ToLower(part))
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
