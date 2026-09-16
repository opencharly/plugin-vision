package vision

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This plugin serves `verb:vision` and validates an authored `vision:` step
// against its OWN self-contained #VisionInput before dispatching. The #LLM*
// blocks in schema/vision.cue MIRROR spec/schema/llm.cue, and that duplication is
// sanctioned — this file must compile STANDALONE (the SDK compiles it serve-side;
// cue exp gengotypes reads it), while spec's defs must generate Go. Neither can be
// the other. The shipped precedent is plugin-desktop-kind's schema/desktop.cue vs
// spec's #Theme/#Session (and plugin-distro's #DistroInput).
//
// What keeps two copies safe is this test: the mirror can DRIFT, and when it does
// the failure is confusing (charly's base accepts a field and this plugin rejects
// it with `#LLM...: field not allowed`, which reads like the field does not
// exist). It asserts the TOP-LEVEL field set of each mirrored def, compared
// against the field list read from the ACTUAL spec schema source — so a field
// added to spec/schema/llm.cue and not mirrored here fails HERE, next to the fix.
func TestVisionSchemaMirrorsSpecLLMDefs(t *testing.T) {
	local, err := os.ReadFile(filepath.Join("schema", "vision.cue"))
	if err != nil {
		t.Fatalf("reading this plugin's schema: %v", err)
	}
	// spec is a module dependency; its schema source is a sibling of the module
	// root. Resolve it via the module cache path from go.mod rather than a
	// hardcoded absolute path.
	specSchema := readSpecLLMSchema(t)

	for _, def := range []string{"#LLMSpec", "#LLMParams", "#LLMResponseFormat", "#LLMReasoning", "#LLMStreamOptions"} {
		got := topLevelFields(t, string(local), def)
		want := topLevelFields(t, specSchema, def)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s drifted from spec's copy.\n got: %v\nwant: %v\n"+
				"If spec added a field, mirror it here; if spec removed one, remove it here.", def, got, want)
		}
	}
}

// topLevelFields returns the sorted top-level field names of a CUE def, ignoring
// comments and NESTED braces.
//
// The body is BOUNDED: the scan starts at the def's first '{' and stops when that
// brace closes (depth returns to 0 after having been >0). A parser that walked to
// EOF would collect the following defs' fields too — returning the SAME inflated
// set for both sides, which compares equal and can never catch drift (the bug in
// the first draft of this test: a dropped `seed` still "passed").
func topLevelFields(t *testing.T, src, def string) []string {
	t.Helper()
	idx := strings.Index(src, def+":")
	if idx < 0 {
		t.Fatalf("def %s not found in source", def)
	}
	body := src[idx:]
	start := strings.Index(body, "{")
	if start < 0 {
		t.Fatalf("def %s has no body", def)
	}
	depth := 0
	fieldRe := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\??\s*:`)
	set := map[string]bool{}
	opened := false
	for _, line := range strings.Split(body[start:], "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") && trimmed != "" {
			// A field at the body's OWN depth (1 while we are inside the body brace).
			if depth == 1 {
				if m := fieldRe.FindStringSubmatch(trimmed); m != nil {
					set[m[1]] = true
				}
			}
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth > 0 {
			opened = true
		}
		if opened && depth == 0 {
			break // the def body closed — stop before the next def
		}
	}
	if len(set) == 0 {
		t.Fatalf("no fields parsed for %s (parser drift?)", def)
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readSpecLLMSchema reads spec/schema/llm.cue from the resolved spec module. It
// finds the module in the Go module cache via the standard layout, falling back to
// a sibling checkout (the umbrella's spec/) so the test runs in both.
func readSpecLLMSchema(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "spec", "schema", "llm.cue"), // umbrella sibling
	}
	if mod := os.Getenv("CHARLY_SPEC_SCHEMA"); mod != "" {
		candidates = append([]string{mod}, candidates...)
	}
	for _, c := range candidates {
		if b, err := os.ReadFile(c); err == nil {
			return string(b)
		}
	}
	t.Skip("spec/schema/llm.cue not reachable from this checkout (set CHARLY_SPEC_SCHEMA to enable the parity check)")
	return ""
}
