// The `vision:` check verb's OWN CUE schema — validate an image (a screenshot the
// check itself produced) by sending it DIRECTLY to an OpenAI-compatible vision
// endpoint from charly.
//
// The #VisionInput def is a mirror of what the plugin's Go params decode; it is
// the SINGLE SOURCE for this plugin's params (spec/schemaparams.Generate, reached
// from a shell as `charly candy params plugin-vision` → params/cue_types_gen.go)
// AND the runtime validation contract the host composes per unit
// (spec/schemacompose.ServedText) and unifies every authored `vision:` step
// against.
//
// ONE NAME, ONE HOME. The shared LLM vocabulary — #LLMSpec, #LLMParams and their
// four companions (#LLMResponseFormat, #LLMReasoning, #LLMStreamOptions,
// #LLMNamedToolChoice) — is the CONTRACT module's
// (`github.com/opencharly/spec/schema`): it is the vocabulary BOTH this verb and
// the pipeline engine's agent stages author against, and it is the def that
// generates the Go types (spec.LLMSpec / spec.LLMParams) the shared SDK `llmkit`
// client consumes. This schema IMPORTS it; the structurally identical local copies
// it used to carry are DELETED, and the parity test that existed only to keep the
// copy honest retires with them (R2 — one name, one home).
//
// #VisionInput and #VisionExpect are this plugin's OWN authoring shapes and stay
// here: #VisionInput is the def the plugin advertises as its InputDef and the only
// def the host's input validator is keyed on.
//
// THE IMPORT RESOLVES OFFLINE. It is the shape spec/schemacompose exists for: the
// contract module is served from the spec Go module the binary already links — no
// network, no module cache, no `cue.mod`, no published CUE module — on the serve
// side (BuildCapabilities → ServedSchema) AND on the host side, which composes the
// SAME body under the DERIVED module declaration (the wire carries no module
// declaration, so both sides evaluate under one they can derive).

import "github.com/opencharly/spec/schema"

// #VisionInput is the `vision:` verb's plugin_input.
//
// The verb asserts something about an IMAGE by asking a vision model. The image
// is read from `artifact` — the SAME path convention the recording/screenshot
// verbs already use (wl:/vnc:/cdp:/spice: screenshot write a PNG to the host
// artifact path), so a check reads the screenshot it just captured with no new
// plumbing. The model's reply is asserted with the shared matcher modifiers (the
// step's stdout matchers match the reply text) and/or a json_schema response.
#VisionInput: {
	// prompt: what to ask about the image. REQUIRED — a vision assertion with no
	// question has no semantics.
	prompt!: string
	// artifact: the image file to validate (host path). The plugin reads it and
	// sends it as a base64 data URL (ollama's OpenAI layer accepts base64 data
	// URLs only — remote image URLs are unsupported there).
	artifact?: string
	// images: ADDITIONAL image files to send alongside `artifact` (a multi-image
	// comparison, e.g. before/after screenshots).
	images?: [...string]
	// mime: the data-URL mime type for the images. Default "image/png".
	mime?: *"image/png" | "image/jpeg" | "image/webp" | "image/gif"
	// llm: the endpoint + request config (the contract's #LLMSpec). Resolution is
	// field-wise: env (EVAL_LLM_*) > this block > the built-in local-ollama
	// default, so a check needs no authored block to run.
	llm?: schema.#LLMSpec @go(Llm)
	// params: a convenience overlay of the contract's #LLMParams onto llm.params
	// (the vision step's own request knobs, e.g. max_tokens/reasoning_effort).
	// Merged field-wise over llm.params.
	params?: schema.#LLMParams @go(Params)
	// expect: an optional reply assertion. The model's text reply is matched with
	// the shared matcher semantics (contains/matches/regex), so a check can assert
	// "the screenshot shows the new layout" without a json_schema round-trip.
	// The step's stdout: matchers are applied to the reply too — this is the
	// vision-specific convenience form.
	expect?: #VisionExpect
}

// #VisionExpect — the reply assertion. Every declared field must hold (AND).
#VisionExpect: {
	// contains: the reply must contain this substring (case-insensitive).
	contains?: string
	// matches: the reply must match this RE2 regular expression.
	matches?: string
	// not_contains: the reply must NOT contain this substring (case-insensitive).
	not_contains?: string
}
