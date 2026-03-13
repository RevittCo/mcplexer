package codemode

import (
	"strings"
	"testing"
)

func TestStripTypeScript_InterfaceBlocks(t *testing.T) {
	code := `interface Params {
  owner: string;
  repo: string;
}
const result = github.list_issues({ owner: "org", repo: "app" });`

	got := StripTypeScript(code)

	if strings.Contains(got, "interface") {
		t.Error("interface block should be removed")
	}
	if !strings.Contains(got, "const result = github.list_issues") {
		t.Error("function call should be preserved")
	}
}

func TestStripTypeScript_TypeAnnotations(t *testing.T) {
	code := `const x: string = "hello";
const y: number = 42;
const z: boolean = true;`

	got := StripTypeScript(code)

	if strings.Contains(got, ": string") {
		t.Errorf("type annotation not stripped: %s", got)
	}
	if !strings.Contains(got, `const x = "hello"`) {
		t.Errorf("expected clean assignment, got: %s", got)
	}
}

func TestStripTypeScript_AsCast(t *testing.T) {
	code := `const data = result as MyType;`

	got := StripTypeScript(code)

	if strings.Contains(got, "as MyType") {
		t.Errorf("as cast not stripped: %s", got)
	}
	if !strings.Contains(got, "const data = result;") {
		t.Errorf("expected clean assignment, got: %s", got)
	}
}

func TestStripTypeScript_DeclareKeyword(t *testing.T) {
	code := `declare namespace github {
  function list_issues(): any;
}
const x = github.list_issues();`

	got := StripTypeScript(code)

	if strings.Contains(got, "declare") {
		t.Errorf("declare keyword not stripped: %s", got)
	}
	if !strings.Contains(got, "const x = github.list_issues()") {
		t.Errorf("function call should be preserved, got: %s", got)
	}
}

func TestStripTypeScript_PreservesLogic(t *testing.T) {
	code := `const issues = github.list_issues({ owner: "org", repo: "app" });
const bugs = issues.filter(i => i.labels.includes("bug"));
for (const bug of bugs) {
  linear.create_issue({ title: bug.title, teamId: "ENG" });
}
print(bugs.length + " bugs synced");`

	got := StripTypeScript(code)

	if !strings.Contains(got, "github.list_issues") {
		t.Error("expected github.list_issues call")
	}
	if !strings.Contains(got, "issues.filter") {
		t.Error("expected filter call")
	}
	if !strings.Contains(got, "linear.create_issue") {
		t.Error("expected linear.create_issue call")
	}
	if !strings.Contains(got, `print(bugs.length + " bugs synced")`) {
		t.Error("expected print call")
	}
}

func TestStripTypeScript_EmptyInput(t *testing.T) {
	got := StripTypeScript("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStripTypeScript_PureJS(t *testing.T) {
	code := `const x = 42;
const y = x * 2;
print(y);`

	got := StripTypeScript(code)

	if !strings.Contains(got, "const x = 42") {
		t.Error("pure JS should be preserved")
	}
}

func TestStripTypeScript_PreservesObjectLiterals(t *testing.T) {
	// This was the critical bug: the regex was stripping object literal values
	// like { filters: { ... } } because it matched `: { ... }` as a type annotation.
	code := `const result = clickup.clickup_search({
  filters: {
    asset_types: ["task"],
    task_statuses: ["unstarted", "active"]
  },
  sort: [{ field: "updated_at", direction: "desc" }],
  count: 50
});`

	got := StripTypeScript(code)

	if !strings.Contains(got, `filters: {`) {
		t.Errorf("object literal value was stripped: %s", got)
	}
	if !strings.Contains(got, `asset_types: ["task"]`) {
		t.Errorf("nested object was stripped: %s", got)
	}
	if !strings.Contains(got, `sort: [{ field:`) {
		t.Errorf("array of objects was stripped: %s", got)
	}
	if !strings.Contains(got, `count: 50`) {
		t.Errorf("simple value was stripped: %s", got)
	}
}

func TestStripTypeScript_InterfaceWithNestedBraces(t *testing.T) {
	// Regression: reInterface regex must handle inline object types within interfaces.
	code := `interface CreateParams {
  config: { name: string; count: number };
  title: string;
}
const x = api.create({ config: { name: "test", count: 1 }, title: "hi" });`

	got := StripTypeScript(code)

	if strings.Contains(got, "interface") {
		t.Errorf("interface block not fully removed: %s", got)
	}
	// The orphaned `;\n}` from partial regex match would cause this to fail.
	if !strings.Contains(got, `const x = api.create`) {
		t.Errorf("function call should be preserved, got: %s", got)
	}
	// Must not contain orphaned closing brace from interface.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "}" {
			t.Errorf("orphaned closing brace found: %s", got)
		}
	}
}

func TestStripTypeScript_PreservesNestedObjects(t *testing.T) {
	code := `const params = { name: "test", config: { enabled: true, tags: ["a", "b"] } };`

	got := StripTypeScript(code)

	if !strings.Contains(got, `config: { enabled: true`) {
		t.Errorf("nested object literal stripped: %s", got)
	}
}
