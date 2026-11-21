package main

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

func collect(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "vendor" || base == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var texts []string
		switch {
		case strings.HasSuffix(path, ".go"):
			texts, err = goComments(path, src)
		case strings.HasSuffix(path, ".py"):
			texts = pyComments(string(src))
		case strings.HasSuffix(path, ".sql"):
			texts = sqlComments(string(src))
		default:
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		for _, text := range texts {
			out = append(out, rel+"\t"+escape(text))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func goComments(path string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, group := range file.Comments {
		var lines []string
		for _, c := range group.List {
			lines = append(lines, rawComment(c.Text))
		}
		text := strings.TrimSpace(strings.Join(lines, "\n"))
		if exemptGo(text) {
			continue
		}
		out = append(out, text)
	}
	return out, nil
}

func rawComment(text string) string {
	switch {
	case strings.HasPrefix(text, "//"):
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, " ")
	case strings.HasPrefix(text, "/*"):
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)
	}
	return strings.TrimRight(text, "\n")
}

func exemptGo(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "go:") || strings.HasPrefix(line, "+build") || strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "line ") {
			continue
		}
		return false
	}
	return true
}

func pyComments(src string) []string {
	b := []byte(src)
	var out []string
	i := 0
	line := 1
	atStart := true
	for i < len(b) {
		if b[i] == '\n' {
			line++
			i++
			atStart = true
			continue
		}
		if b[i] == ' ' || b[i] == '\t' || b[i] == '\r' {
			i++
			continue
		}
		if atStart && line == 1 && i+1 < len(b) && b[i] == '#' && b[i+1] == '!' {
			for i < len(b) && b[i] != '\n' {
				i++
			}
			continue
		}
		atStart = false
		if b[i] == '#' {
			j := i + 1
			for j < len(b) && b[j] != '\n' {
				j++
			}
			text := strings.TrimSpace(string(b[i+1 : j]))
			if !strings.Contains(text, "coding:") && !strings.Contains(text, "coding=") {
				out = append(out, text)
			}
			i = j
			continue
		}
		if next, ok := skipPyString(b, i); ok {
			i = next
			continue
		}
		i++
	}
	return out
}

func skipPyString(b []byte, i int) (int, bool) {
	j := i
	for j < len(b) && strings.ContainsRune("rRuUbBfF", rune(b[j])) {
		j++
	}
	if j == i || j >= len(b) || (b[j] != '\'' && b[j] != '"') {
		if j < len(b) && (b[j] == '\'' || b[j] == '"') {
			return consumePyQuotes(b, j, false)
		}
		return 0, false
	}
	raw := false
	for k := i; k < j; k++ {
		if b[k] == 'r' || b[k] == 'R' {
			raw = true
		}
	}
	return consumePyQuotes(b, j, raw)
}

func consumePyQuotes(b []byte, j int, raw bool) (int, bool) {
	quote := b[j]
	triple := j+2 < len(b) && b[j+1] == quote && b[j+2] == quote
	if triple {
		j += 3
		for j+2 < len(b) {
			if !raw && b[j] == '\\' {
				j += 2
				continue
			}
			if b[j] == quote && b[j+1] == quote && b[j+2] == quote {
				return j + 3, true
			}
			j++
		}
		return len(b), true
	}
	j++
	for j < len(b) {
		if !raw && b[j] == '\\' {
			j += 2
			continue
		}
		if b[j] == quote || b[j] == '\n' {
			if b[j] == quote {
				j++
			}
			return j, true
		}
		j++
	}
	return len(b), true
}

func sqlComments(src string) []string {
	b := []byte(src)
	var out []string
	inSingle := false
	for i := 0; i < len(b); i++ {
		if inSingle {
			if b[i] == '\'' {
				if i+1 < len(b) && b[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if b[i] == '\'' {
			inSingle = true
			continue
		}
		if i+1 < len(b) && b[i] == '-' && b[i+1] == '-' {
			j := i + 2
			for j < len(b) && b[j] != '\n' {
				j++
			}
			out = append(out, strings.TrimSpace(string(b[i+2:j])))
			i = j - 1
			continue
		}
		if i+1 < len(b) && b[i] == '/' && b[i+1] == '*' {
			j := i + 2
			for j+1 < len(b) && !(b[j] == '*' && b[j+1] == '/') {
				j++
			}
			end := j
			if j+1 < len(b) {
				j += 2
			} else {
				j = len(b)
			}
			out = append(out, strings.TrimSpace(string(b[i+2:end])))
			i = j - 1
		}
	}
	return out
}

func escape(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`)
	return replacer.Replace(text)
}

func baselinePath(root string) string {
	return filepath.Join(root, "tools", "commentlint", "baseline.txt")
}

func readBaseline(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseBaseline(raw), nil
}

func parseBaseline(raw []byte) []string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

func writeBaseline(path string, entries []string) error {
	sort.Strings(entries)
	var b strings.Builder
	b.WriteString("# Comment allowlist. The count may only shrink. Run: go run ./tools/commentlint --write\n")
	for _, entry := range entries {
		b.WriteString(entry)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func isSubset(current, allowed []string) bool {
	have := map[string]int{}
	for _, line := range allowed {
		have[line]++
	}
	for _, line := range current {
		if have[line] == 0 {
			return false
		}
		have[line]--
	}
	return true
}

func reportMismatch(current, base []string) string {
	have := map[string]int{}
	for _, line := range base {
		have[line]++
	}
	var extra []string
	for _, line := range current {
		if have[line] == 0 {
			extra = append(extra, line)
			continue
		}
		have[line]--
	}
	var missing []string
	for line, n := range have {
		for i := 0; i < n; i++ {
			missing = append(missing, line)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	var b strings.Builder
	if len(extra) > 0 {
		b.WriteString("new comments:\n")
		for _, line := range extra {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if len(missing) > 0 {
		b.WriteString("baseline entries no longer present (shrink the baseline with --write):\n")
		for _, line := range missing {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	fmt.Fprintf(&b, "current %d, baseline %d\n", len(current), len(base))
	return b.String()
}
