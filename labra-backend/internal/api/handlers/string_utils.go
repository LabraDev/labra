package handlers

import "strings"

func slugify(in string) string {
	v := strings.TrimSpace(strings.ToLower(in))
	if v == "" {
		return "app"
	}
	v = strings.ReplaceAll(v, " ", "-")
	v = strings.ReplaceAll(v, "_", "-")
	return v
}
