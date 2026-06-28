package database

import "strings"

func splitMigrationStatements(sql string) []string {
	var out []string
	var buf strings.Builder
	inSingle := false
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if !inSingle && c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			continue
		}
		if c == '\'' {
			if inSingle && i+1 < len(sql) && sql[i+1] == '\'' {
				buf.WriteByte(c)
				buf.WriteByte(sql[i+1])
				i++
				continue
			}
			inSingle = !inSingle
			buf.WriteByte(c)
			continue
		}
		if c == ';' && !inSingle {
			stmt := strings.TrimSpace(buf.String())
			if stmt != "" {
				out = append(out, stmt)
			}
			buf.Reset()
			continue
		}
		buf.WriteByte(c)
	}
	if tail := strings.TrimSpace(buf.String()); tail != "" {
		out = append(out, tail)
	}
	return out
}
