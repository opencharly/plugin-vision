package vision

// schema_test.go — the served-schema contract of THIS plugin, proved at the two
// boundaries the contract actually has.
//
// The repo's other gates do not cover this: `charly box validate` checks the candy
// manifest and its `plugin:` block — and it does NOT compile the served schema, so
// it stays green over a dangling definition AND over an unresolvable import —
// while `go test` of the verb exercises the provider, never the schema. A plugin
// whose served schema stopped composing, or whose `#VisionInput` stopped
// referencing the contract's `#LLMSpec` / `#LLMParams`, would keep every one of
// those green while failing at start-up on the host. So this file composes the
// schema the way each half of the contract composes it, and judges the SAME
// authored documents through both:
//
//   - SERVE side — `NewMeta().Describe` → `sdk.BuildCapabilities` →
//     `schemacompose.ServedSchema`, exactly the call the provider makes when it
//     advertises verb:vision;
//   - HOST side — the `Capabilities.schema_cue` body that travelled over the wire,
//     composed by `schemacompose.ServedText` under the DERIVED module declaration
//     (charly's registerPluginUnitSchema calls the same function on that same
//     text).
//
// The fixture is the equivalence record AND the repo's own authored documents: the
// README's two examples and the org's PR-eval step, plus the shapes this plugin
// must keep rejecting. It is the same document set the cutover measured against
// `origin/main`'s schema, where ZERO verdicts moved — six accepted, eleven
// rejected, both directions.
//
// The mirror this file replaces (schema_parity_test.go) existed only to keep the
// local copy of the contract's six LLM defs honest. The import makes drift
// impossible, so the guard retires with the copy: there is nothing left to guard.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/opencharly/plugin-vision/candy/plugin-vision/params"
	pb "github.com/opencharly/spec/proto"
	"github.com/opencharly/spec/schemacompose"
	"github.com/opencharly/spec/spec"
)

// The plugin's params must carry the CONTRACT's wire types, not copies of them:
// these assignments fail to compile if the schema ever stops importing
// schema.#LLMSpec / schema.#LLMParams and goes back to declaring local twins.
var (
	_ spec.LLMSpec   = params.VisionInput{}.Llm
	_ spec.LLMParams = params.VisionInput{}.Params
)

const visionInputDef = "#VisionInput"

// llmVocabularyDefs are the six definitions that moved to the contract module.
// None of them may be declared by this plugin's schema any more.
var llmVocabularyDefs = []string{
	"#LLMSpec",
	"#LLMParams",
	"#LLMResponseFormat",
	"#LLMReasoning",
	"#LLMStreamOptions",
	"#LLMNamedToolChoice",
}

// servedUnit composes the plugin's embedded schema the way the SERVE side does.
func servedUnit(t *testing.T) cue.Value {
	t.Helper()
	v, err := schemacompose.ServedSchema(nil, schemaFS, "schema")
	if err != nil {
		t.Fatalf("the serve side cannot compose candy/plugin-vision/schema: %v", err)
	}
	return v
}

// hostUnit takes the capabilities the provider actually advertises over Describe,
// then composes the schema body the host receives the way the HOST gate does.
func hostUnit(t *testing.T) (string, cue.Value) {
	t.Helper()
	caps, err := NewMeta().Describe(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if caps == nil {
		t.Fatal("Describe returned no capabilities")
	}
	body := caps.GetSchemaCue()
	if strings.TrimSpace(body) == "" {
		t.Fatal("the plugin advertises an EMPTY schema_cue")
	}
	v, err := schemacompose.ServedText(nil, body)
	if err != nil {
		t.Fatalf("the host side cannot compose the served schema_cue: %v", err)
	}
	return body, v
}

// topLevelDefs returns the top-level definition names a composed unit declares.
func topLevelDefs(t *testing.T, v cue.Value) map[string]bool {
	t.Helper()
	it, err := v.Fields(cue.Definitions(true))
	if err != nil {
		t.Fatalf("reading the composed unit's definitions: %v", err)
	}
	out := map[string]bool{}
	for it.Next() {
		out[it.Selector().String()] = true
	}
	return out
}

// judge applies the SAME rule the host's validator applies (plugin_loader.go:
// `def.Unify(input).Validate(cue.Concrete(true))`) and reports "<nil>" for an
// accepted document.
func judge(t *testing.T, unit cue.Value, doc string) string {
	t.Helper()
	d := unit.LookupPath(cue.ParsePath(visionInputDef))
	if d.Err() != nil {
		t.Fatalf("composed unit has no %s: %v", visionInputDef, d.Err())
	}
	in := cuecontext.New().CompileBytes([]byte(doc))
	if in.Err() != nil {
		t.Fatalf("fixture document does not compile: %v\n%s", in.Err(), doc)
	}
	if err := d.Unify(in).Validate(cue.Concrete(true)); err != nil {
		return err.Error()
	}
	return "<nil>"
}

// visionInputFixture is the equivalence record: the repo's OWN authored documents
// (the README's usage and endpoint examples) plus the org's PR-eval `vision:` step,
// plus every shape the plugin's input def accepted or rejected before the six LLM
// defs moved behind the import.
var visionInputFixture = []struct {
	name  string
	doc   string
	valid bool
}{
	{
		name:  "README usage example (panel/clock assertion)",
		doc:   `{"prompt":"Does this screenshot show a top panel with a clock on the right?","artifact":"/tmp/shot.png","expect":{"contains":"yes"}}`,
		valid: true,
	},
	{
		name:  "README endpoint block (llm + params)",
		doc:   `{"prompt":"...","artifact":"/tmp/shot.png","llm":{"base_url":"http://localhost:11434/v1","model":"qwen3-vl:8b","api_key":"$env.OPENAI_API_KEY","params":{"max_tokens":300,"reasoning_effort":"none"}}}`,
		valid: true,
	},
	{
		name:  "eval-omarchy PR-eval step (YES/NO judgement)",
		doc:   `{"prompt":"Reply with ONLY the single word YES if this screenshot shows a rendered Linux desktop (a panel/bar and a wallpaper) and NOT a blank, black, or error screen; reply NO otherwise.","artifact":"/tmp/pr-12138.png","expect":{"contains":"yes"}}`,
		valid: true,
	},
	{
		name:  "multi-image step with json_schema reply and the params overlay",
		doc:   `{"prompt":"compare","artifact":"/tmp/a.png","images":["/tmp/b.png"],"mime":"image/webp","params":{"temperature":0,"top_p":0.1,"seed":7,"response_format":{"type":"json_schema","json_schema":{"name":"verdict","schema":{"type":"object"},"strict":true}},"tool_choice":"auto","extra":{"service_tier":"flex"}}}`,
		valid: true,
	},
	{
		name:  "empty llm block",
		doc:   `{"prompt":"q","llm":{}}`,
		valid: true,
	},
	{
		// The schema admits an empty string; the provider fails it at dispatch
		// (TestInvoke_PromptRequired). Kept in the fixture so the cutover cannot
		// silently tighten a constraint the verb owns.
		name:  "empty prompt (schema admits it; the provider fails it at dispatch)",
		doc:   `{"prompt":""}`,
		valid: true,
	},
	{
		name:  "missing prompt",
		doc:   `{"artifact":"/tmp/shot.png"}`,
		valid: false,
	},
	{
		name:  "unknown key at the input level",
		doc:   `{"prompt":"q","bogus":1}`,
		valid: false,
	},
	{
		name:  "unknown key inside llm",
		doc:   `{"prompt":"q","llm":{"bogus":1}}`,
		valid: false,
	},
	{
		name:  "unknown key inside expect",
		doc:   `{"prompt":"q","expect":{"bogus":1}}`,
		valid: false,
	},
	{
		name:  "string max_retries",
		doc:   `{"prompt":"q","llm":{"max_retries":"2"}}`,
		valid: false,
	},
	{
		name:  "string artifact",
		doc:   `{"prompt":"q","artifact":1}`,
		valid: false,
	},
	{
		name:  "string images",
		doc:   `{"prompt":"q","images":"a.png"}`,
		valid: false,
	},
	{
		name:  "bad mime",
		doc:   `{"prompt":"q","mime":"image/tiff"}`,
		valid: false,
	},
	{
		name:  "bad reasoning_effort",
		doc:   `{"prompt":"q","llm":{"params":{"reasoning_effort":"extreme"}}}`,
		valid: false,
	},
	{
		name:  "temperature out of range",
		doc:   `{"prompt":"q","params":{"temperature":3}}`,
		valid: false,
	},
	{
		name:  "string expect",
		doc:   `{"prompt":"q","expect":"yes"}`,
		valid: false,
	},
}

// TestServedSchemaLoadsAndCarriesTheImport is the load proof R7 asks for at this
// boundary: the schema COMPOSES as one CUE module instance (the serve-side compose
// fails loudly if the import cannot resolve), the wire body carries that import,
// and the definitions this plugin declares are its OWN two — the six the contract
// owns are gone rather than kept beside the import.
func TestServedSchemaLoadsAndCarriesTheImport(t *testing.T) {
	serveDefs := topLevelDefs(t, servedUnit(t))
	if !serveDefs[visionInputDef] {
		t.Fatalf("the served schema declares no %s: %v", visionInputDef, serveDefs)
	}
	for _, deleted := range llmVocabularyDefs {
		if serveDefs[deleted] {
			t.Errorf("the served schema still DECLARES %s — the restated twin must be deleted, not kept beside the import", deleted)
		}
	}

	body, host := hostUnit(t)
	if !strings.Contains(body, `import "github.com/opencharly/spec/schema"`) {
		t.Errorf("the served schema_cue does not import the contract module:\n%s", body)
	}
	hostDefs := topLevelDefs(t, host)
	if !hostDefs[visionInputDef] {
		t.Fatalf("the host-composed unit declares no %s: %v", visionInputDef, hostDefs)
	}
	for _, deleted := range llmVocabularyDefs {
		if hostDefs[deleted] {
			t.Errorf("the host-composed unit DECLARES %s — the wire body must carry only the plugin's own defs", deleted)
		}
	}
}

// TestVisionInputFixtureJudgedIdenticallyByBothHalves is the equivalence proof:
// the authored documents are judged by the value the serve side composes AND by
// the value the host composes from the wire, both must agree, and both must agree
// with the verdict the plugin gave before the cutover (six accepted, eleven
// rejected).
func TestVisionInputFixtureJudgedIdenticallyByBothHalves(t *testing.T) {
	serve := servedUnit(t)
	_, host := hostUnit(t)

	accepted, rejected := 0, 0
	for _, tc := range visionInputFixture {
		gotServe := judge(t, serve, tc.doc)
		gotHost := judge(t, host, tc.doc)
		t.Logf("%-58s accept=%v verdict=%s", tc.name, tc.valid, gotServe)
		if gotServe != gotHost {
			t.Errorf("%s: the two halves disagree\n serve: %s\n host:  %s\n doc: %s", tc.name, gotServe, gotHost, tc.doc)
		}
		if tc.valid {
			accepted++
			if gotServe != "<nil>" {
				t.Errorf("%s must be ACCEPTED, got: %s", tc.name, gotServe)
			}
			continue
		}
		rejected++
		if gotServe == "<nil>" {
			t.Errorf("%s must be REJECTED, but the schema accepted it", tc.name)
		}
	}
	if accepted != 6 || rejected != 11 {
		t.Fatalf("the fixture must keep its measured shape (6 accept / 11 reject), got %d/%d", accepted, rejected)
	}
	t.Logf("fixture: %d accepted, %d rejected, both halves identical", accepted, rejected)
}

// TestImportIsEnforcedByTheImportedDefinition proves the import is LIVE rather
// than decorative: documents that violate the CONTRACT's own constraints are
// rejected with the imported definition's error, while conforming ones pass.
func TestImportIsEnforcedByTheImportedDefinition(t *testing.T) {
	serve := servedUnit(t)
	for _, tc := range []struct{ name, doc string }{
		{"llm.max_retries is an int in the contract", `{"prompt":"q","llm":{"max_retries":"2"}}`},
		{"llm.params.temperature is bounded 0..2 in the contract", `{"prompt":"q","llm":{"params":{"temperature":9}}}`},
		{"llm.params.reasoning_effort is the contract's disjunction", `{"prompt":"q","llm":{"params":{"reasoning_effort":"extreme"}}}`},
		{"llm is closed by the contract", `{"prompt":"q","llm":{"bogus":1}}`},
	} {
		if got := judge(t, serve, tc.doc); got == "<nil>" {
			t.Errorf("%s: a config violating the IMPORTED #LLMSpec must be rejected", tc.name)
		}
	}
	if got := judge(t, serve, `{"prompt":"q","llm":{"base_url":"http://localhost:11434/v1","model":"m","params":{"temperature":0.5,"seed":1}}}`); got != "<nil>" {
		t.Fatalf("a conforming llm block must pass: %s", got)
	}
}

// TestParamsTypesAreTheContractTypes states the same relation in one place a
// reader can read without a compiler: the generated fields ARE the contract's
// types, and their JSON keys are unchanged.
func TestParamsTypesAreTheContractTypes(t *testing.T) {
	var in params.VisionInput
	in.Llm = spec.LLMSpec{Model: "m"}
	in.Params = spec.LLMParams{User: "u"}
	if got := fmt.Sprintf("%T", in.Llm); got != "spec.LLMSpec" {
		t.Fatalf("params.VisionInput.Llm is %s, want spec.LLMSpec", got)
	}
	if got := fmt.Sprintf("%T", in.Params); got != "spec.LLMParams" {
		t.Fatalf("params.VisionInput.Params is %s, want spec.LLMParams", got)
	}
}
