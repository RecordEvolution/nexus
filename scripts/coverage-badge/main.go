// Command coverage-badge rewrites the Coverage badge line in README.md to
// a shields.io static-badge URL with the current coverage percentage.
// Stdlib only.
//
// Usage: go run ./scripts/coverage-badge
//
// Input:
//   coverage.txt — single line containing a percentage like "57.4%"
//                  (produced by `go tool cover -func=coverage.out | tail -1 | awk '{print $3}'`)
//
// Output:
//   README.md — the line `![Coverage](...)` is rewritten in place. shields.io
//   renders the badge on the fly when the README is viewed, so there's no
//   committed SVG to maintain. Idempotent: same percentage → no diff.
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	coverageFile = "coverage.txt"
	readmeFile   = "README.md"

	travisLineRe = `^\[\!\[Build Status\]\(https://travis-ci\.org/[^)]+\)\]\(https://travis-ci\.org/[^)]+\)\s*$`
	// Match any existing Coverage badge line, regardless of badge URL —
	// catches both shields.io URLs and the older `doc/coverage.svg` form.
	coverageLine = `^\[\!\[Coverage\]\([^)]*\)\]\([^)]*\)\s*$`
	ciLine       = "[![Main CI](https://github.com/gammazero/nexus/actions/workflows/main-golint.yml/badge.svg)](https://github.com/gammazero/nexus/actions/workflows/main-golint.yml)"
)

func main() {
	pctRaw, err := os.ReadFile(coverageFile)
	if err != nil {
		log.Fatalf("read %s: %v", coverageFile, err)
	}
	pctStr := strings.TrimSpace(string(pctRaw))
	pctStr = strings.TrimSuffix(pctStr, "%")
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		log.Fatalf("parse coverage %q: %v", pctStr, err)
	}

	badge := fmt.Sprintf(
		"[![Coverage](https://img.shields.io/badge/coverage-%.1f%%25-%s)](https://github.com/gammazero/nexus/actions/workflows/main-golint.yml)",
		pct, pickColor(pct),
	)

	if err := updateREADME(badge); err != nil {
		log.Fatalf("update %s: %v", readmeFile, err)
	}

	fmt.Printf("coverage %.1f%% → %s\n", pct, readmeFile)
}

// pickColor maps coverage % to a shields.io named color.
func pickColor(pct float64) string {
	switch {
	case pct < 50:
		return "red"
	case pct < 70:
		return "orange"
	case pct < 80:
		return "yellow"
	case pct < 90:
		return "yellowgreen"
	default:
		return "brightgreen"
	}
}

// updateREADME replaces an existing Coverage badge line, or inserts one
// (and replaces a dead Travis badge if present). Idempotent.
func updateREADME(badge string) error {
	raw, err := os.ReadFile(readmeFile)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")

	travisRe := regexp.MustCompile(travisLineRe)
	covRe := regexp.MustCompile(coverageLine)

	covIdxs := []int{}
	travisIdx := -1
	hasCI := false
	for i, line := range lines {
		switch {
		case covRe.MatchString(line):
			covIdxs = append(covIdxs, i)
		case travisRe.MatchString(line):
			travisIdx = i
		case strings.Contains(line, "actions/workflows/main-golint.yml/badge.svg"):
			hasCI = true
		}
	}

	switch {
	case len(covIdxs) > 0:
		// Refresh the first occurrence; drop any duplicates left by an
		// earlier badge implementation.
		lines[covIdxs[0]] = badge
		for i := len(covIdxs) - 1; i >= 1; i-- {
			idx := covIdxs[i]
			lines = append(lines[:idx], lines[idx+1:]...)
		}

	case travisIdx >= 0:
		// First-time bootstrap from a Travis-era README.
		repl := []string{}
		if !hasCI {
			repl = append(repl, ciLine)
		}
		repl = append(repl, badge)
		lines = append(lines[:travisIdx], append(repl, lines[travisIdx+1:]...)...)

	default:
		// No Travis line, no existing coverage line — insert above License if found.
		inserted := false
		for i, line := range lines {
			if strings.Contains(line, "img.shields.io/badge/License") {
				lines = append(lines[:i], append([]string{badge}, lines[i:]...)...)
				inserted = true
				break
			}
		}
		if !inserted {
			for i, line := range lines {
				if strings.HasPrefix(line, "# ") && i+2 < len(lines) {
					lines = append(lines[:i+2], append([]string{badge}, lines[i+2:]...)...)
					break
				}
			}
		}
	}

	out := []byte(strings.Join(lines, "\n"))
	if bytes.Equal(out, raw) {
		return nil
	}
	return os.WriteFile(readmeFile, out, 0o644)
}
