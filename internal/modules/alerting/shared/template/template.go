package template

import (
	"strconv"
	"strings"
)

type Vars struct {
	Values     map[string]string
	IsAlert    bool
	IsWarning  bool
	IsRecovery bool
}

func Render(body string, v Vars) string {
	out := body
	out = renderSections(out, "is_alert", v.IsAlert)
	out = renderSections(out, "is_warning", v.IsWarning)
	out = renderSections(out, "is_recovery", v.IsRecovery)
	out = renderScalars(out, v.Values)
	return out
}

func renderSections(body, tag string, keep bool) string {
	openTag := "{{#" + tag + "}}"
	closeTag := "{{/" + tag + "}}"
	for {
		before, rest, found := strings.Cut(body, openTag)
		if !found {
			return body
		}
		inner, after, closed := strings.Cut(rest, closeTag)
		if !closed {
			return body
		}
		if !keep {
			inner = ""
		}
		body = before + inner + after
	}
}

func renderScalars(body string, values map[string]string) string {
	out := strings.Builder{}
	out.Grow(len(body))
	i := 0
	for i < len(body) {
		if i+1 < len(body) && body[i] == '{' && body[i+1] == '{' {
			end := strings.Index(body[i:], "}}")
			if end < 0 {
				out.WriteString(body[i:])
				return out.String()
			}
			key := strings.TrimSpace(body[i+2 : i+end])

			if strings.HasPrefix(key, "#") || strings.HasPrefix(key, "/") {
				out.WriteString(body[i : i+end+2])
				i += end + 2
				continue
			}
			if v, ok := values[key]; ok {
				out.WriteString(v)
			}
			i += end + 2
			continue
		}
		out.WriteByte(body[i])
		i++
	}
	return out.String()
}

// FormatFloat renders v in its shortest exact form.
func FormatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
