package codemode

import (
	"regexp"
	"strings"
)

// Compiled patterns for stripping TypeScript type annotations.
// These are intentionally conservative — only matching patterns that are
// unambiguously TypeScript and never valid JavaScript.
var (
	// Matches `interface Name { ... }` blocks (including multiline).
	// Uses a greedy match to handle nested braces from inline object types
	// like `config: { name: string; count: number }`.
	// Safe: `interface {}` is never valid JS.
	reInterface = regexp.MustCompile(`(?ms)^\s*interface\s+\w+\s*\{.*?^\s*\}\s*\n?`)

	// Matches type annotations ONLY after const/let/var declarations:
	//   const x: string = ...  →  const x = ...
	//   let y: number;         →  let y;
	// This avoids matching object property values like { key: { ... } }.
	reVarTypeAnnotation = regexp.MustCompile(
		`((?:const|let|var)\s+\w+)` + // captured: declaration
			`:\s*` + // colon + whitespace
			`(?:` +
			`[A-Za-z_][\w.]*(?:<[^>]+>)?(?:\[\])?` + // named types, generics, arrays
			`(?:\s*\|\s*[A-Za-z_][\w.]*(?:<[^>]+>)?(?:\[\])?)*` + // union types
			`)`,
	)

	// Matches `as Type` casts — only when followed by a delimiter.
	// e.g. `result as string;` → `result;`
	reAsCast = regexp.MustCompile(`\s+as\s+[A-Za-z_][\w.]*(?:<[^>]+>)?`)

	// Matches `declare` keyword at start of line (from generated declarations).
	// Safe: `declare` is never valid JS.
	reDeclare = regexp.MustCompile(`(?m)^\s*declare\s+`)

	// Matches generated header comments.
	reGenComment = regexp.MustCompile(`(?m)^//\s*Auto-generated.*\n|^//\s*Tool functions.*\n`)
)

// StripTypeScript removes TypeScript-specific syntax from code, producing
// valid JavaScript that can be executed in Goja. This handles the constrained
// subset of TypeScript that LLMs generate for our code API:
//   - Interface declarations
//   - Type annotations on variable declarations (const x: Type = ...)
//   - Type casts (as Type)
//   - declare keywords
//
// Intentionally does NOT strip `: value` in object literals like { key: value }.
func StripTypeScript(code string) string {
	// Remove interface blocks first.
	code = reInterface.ReplaceAllString(code, "")

	// Remove type annotations on variable declarations only.
	code = reVarTypeAnnotation.ReplaceAllString(code, "$1")

	// Remove `as Type` casts.
	code = reAsCast.ReplaceAllString(code, "")

	// Remove `declare` keyword (keep the rest of the line).
	code = reDeclare.ReplaceAllString(code, "")

	// Remove generated header comments.
	code = reGenComment.ReplaceAllString(code, "")

	// Clean up blank lines.
	code = cleanBlankLines(code)

	return strings.TrimSpace(code)
}

// cleanBlankLines collapses runs of 3+ blank lines into 2.
func cleanBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	blanks := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blanks++
			if blanks <= 2 {
				out = append(out, line)
			}
		} else {
			blanks = 0
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
