// Package render fills {placeholders} in decoded-JSON values.
package render

import (
	"fmt"
	"strings"
)

// Fill returns a copy of v with every string's {name} replaced from vars.
// "{{" and "}}" produce literal braces. Substituted values are inserted
// verbatim and never re-scanned. Non-string scalars pass through unchanged.
func Fill(v any, vars map[string]string) (any, error) {
	switch x := v.(type) {
	case string:
		return fillString(x, vars)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			f, err := Fill(e, vars)
			if err != nil {
				return nil, err
			}
			out[i] = f
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			f, err := Fill(e, vars)
			if err != nil {
				return nil, err
			}
			out[k] = f
		}
		return out, nil
	default:
		return v, nil
	}
}

func fillString(s string, vars map[string]string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '{' && i+1 < len(s) && s[i+1] == '{':
			b.WriteByte('{')
			i++
		case c == '}' && i+1 < len(s) && s[i+1] == '}':
			b.WriteByte('}')
			i++
		case c == '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return "", fmt.Errorf("unclosed placeholder in %q", s)
			}
			name := s[i+1 : i+end]
			val, ok := vars[name]
			if !ok {
				return "", fmt.Errorf("no value for placeholder {%s} (use --set %s=...)", name, name)
			}
			b.WriteString(val)
			i += end
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}
