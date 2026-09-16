// SELF-CONTAINED input schema for the `vision:` check verb — validate an image
// (a screenshot the check itself produced) by sending it DIRECTLY to an
// OpenAI-compatible vision endpoint from charly.
//
// The #VisionInput def is a mirror of what the plugin's Go params decode; it is
// the SINGLE SOURCE for this plugin's params (cue exp gengotypes → params/
// cue_types_gen.go) AND the runtime validation contract the host splices onto the
// base and unifies every authored `vision:` step against.
//
// The LLM blocks (#LLMSpec/#LLMParams and their companions) are duplicated here
// from spec/schema/llm.cue. That duplication is SANCTIONED, not an R3 violation —
// the identical precedent is plugin-desktop-kind's schema/desktop.cue vs spec's
// #Theme/#Session (and plugin-distro's #DistroInput): this file must compile
// STANDALONE (the SDK compiles it serve-side and cue exp gengotypes reads it),
// while spec's defs must generate Go. Neither can be the other. A parity test
// (schema_parity_test.go) makes the next drift fail HERE, next to the fix.

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
	// llm: the endpoint + request config (see #LLMSpec). Resolution is
	// field-wise: env (EVAL_LLM_*) > this block > the built-in local-ollama
	// default, so a check needs no authored block to run.
	llm?: #LLMSpec
	// params: a convenience overlay of #LLMParams onto llm.params (the vision
	// step's own request knobs, e.g. max_tokens/reasoning_effort). Merged
	// field-wise over llm.params.
	params?: #LLMParams
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

// ── the shared LLM vocabulary (mirror of spec/schema/llm.cue) ────────────────

#LLMSpec: close({
	base_url?:     string
	model?:        string
	api_key?:      string
	organization?: string
	project?:      string
	timeout?:      string
	idle_timeout?: string
	max_retries?:  int & >=0 @go(Max_retries,optional=nillable)
	headers?:      {[string]: string}
	params?:       #LLMParams
})

#LLMParams: close({
	temperature?:           number & >=0 & <=2 @go(Temperature,type=*float64)
	top_p?:                 number & >=0 & <=1 @go(Top_p,type=*float64)
	max_tokens?:            int & >0 @go(Max_tokens,optional=nillable)
	max_completion_tokens?: int & >0 @go(Max_completion_tokens,optional=nillable)
	frequency_penalty?:     number & >=-2 & <=2 @go(Frequency_penalty,type=*float64)
	presence_penalty?:      number & >=-2 & <=2 @go(Presence_penalty,type=*float64)
	seed?:                  int @go(Seed,optional=nillable)
	stop?:                  string | [...string]
	response_format?:       #LLMResponseFormat
	reasoning_effort?:      "high" | "medium" | "low" | "none" @go(Reasoning_effort,type=string)
	reasoning?:             #LLMReasoning
	stream_options?:        #LLMStreamOptions
	parallel_tool_calls?:   bool @go(Parallel_tool_calls,optional=nillable)
	tool_choice?:           "none" | "auto" | "required" | #LLMNamedToolChoice
	logprobs?:              bool @go(Logprobs,optional=nillable)
	top_logprobs?:          int @go(Top_logprobs,optional=nillable)
	user?:                  string
	metadata?:              {[string]: string}
	logit_bias?:            {[string]: int}
	extra?:                 {[string]: _}
})

#LLMResponseFormat: close({
	type: "text" | "json_object" | "json_schema" @go(Type,type=string)
	json_schema?: close({
		name:         string
		description?: string
		schema:       {[string]: _}
		strict?:      bool
	})
})

#LLMReasoning: close({
	effort?: "high" | "medium" | "low" | "none" @go(Effort,type=string)
})

#LLMStreamOptions: close({
	include_usage?: bool @go(Include_usage,optional=nillable)
})

#LLMNamedToolChoice: close({
	function: close({name: string})
})
