package views

import (
	"fmt"
	"html"
	"strings"
)

func lenStr[T any](s []T) string {
	return fmt.Sprint(len(s))
}

func safeSnippet(s string) string {
	safe := html.EscapeString(s)
	safe = strings.ReplaceAll(safe, "⟪MARK_START⟫", "<mark>")
	safe = strings.ReplaceAll(safe, "⟪MARK_END⟫", "</mark>")
	return safe
}
