package vision

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/opencharly/plugin-vision/candy/plugin-vision/params"
	"github.com/opencharly/sdk"
	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/sdk/llmkit"
	pb "github.com/opencharly/spec/proto"
	"github.com/opencharly/spec/spec"
)

// provider.go is the out-of-process vision verb provider — charly's host
// dispatches a `vision:` check step to it through the registry
// (ResolveVerb("vision") → this grpcProvider → Provider.Invoke) with the FULL #Op
// marshaled as params_json (the per-verb fields in the desugared plugin_input) and
// a CheckEnv snapshot as env.
//
// Because the out-of-process path does NOT run a host-side matcher pipeline, this
// Invoke OWNS the whole verdict: read the image(s), resolve the endpoint, ask the
// model, then evaluate the reply assertion + the shared stdout/stderr/exit_status
// matchers itself (via sdk.VerbVerdict — R3), returning the wire {status,message}
// the host decodes.

// visionEnv is the plugin-side decode of the CheckEnv the host ships as
// Operation.Env for a `vision:` check step. The verb needs no venue endpoint: it
// reads a HOST-side artifact and talks to the LLM endpoint directly.
type visionEnv struct {
	Box   string `json:"box"`
	Mode  string `json:"mode"` // "live" | "box"
	Venue string `json:"venue"`
}

type provider struct{ pb.UnimplementedProviderServer }

// Invoke runs one `vision:` operation: decode the op + typed input + env, resolve
// the LLM config, read the image(s) into data URLs, ask the model, assert the
// reply, and self-evaluate the matchers.
func (provider) Invoke(ctx context.Context, req *pb.InvokeRequest) (*pb.InvokeReply, error) {
	var op spec.Op
	if len(req.GetParamsJson()) > 0 {
		if err := json.Unmarshal(req.GetParamsJson(), &op); err != nil {
			return sdk.ResultJSON("fail", "vision: decode op: "+err.Error())
		}
	}
	var in params.VisionInput
	kit.DecodeInput(op.PluginInput, &in)
	var env visionEnv
	if len(req.GetEnvJson()) > 0 {
		_ = json.Unmarshal(req.GetEnvJson(), &env)
	}

	// A `vision:` step asserts on an IMAGE; without a prompt there is nothing to
	// ask and without an image there is nothing to look at. Both are hard errors at
	// dispatch (the host's validate-time required-modifier check keyed off the
	// in-proc live-verb seam, which an external verb is not).
	if in.Prompt == "" {
		return sdk.VerbVerdict("vision", "assert", "", fmt.Errorf("prompt is required (what to ask about the image)"), &op, false)
	}
	paths := imagePaths(&in)
	if len(paths) == 0 {
		return sdk.VerbVerdict("vision", "assert", "", fmt.Errorf("artifact (or images) is required — the image file to validate"), &op, false)
	}

	out, runErr := runVision(ctx, &in, paths)
	// The reply IS the verb's stdout, so the shared stdout/stderr/exit_status
	// matcher pipeline grades it exactly like any other verb's output (R3). The
	// `expect:` assertion is evaluated inside runVision and folded into runErr.
	return sdk.VerbVerdict("vision", "assert", out, runErr, &op, false)
}

// imagePaths returns the image files to send: the primary `artifact` first, then
// any `images`, skipping blanks (and de-duplicating an accidental self-repeat).
func imagePaths(in *params.VisionInput) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	add(in.Artifact)
	for _, p := range in.Images {
		add(p)
	}
	return out
}

// runVision reads the images, resolves the endpoint, and asks the model, then
// applies the `expect:` assertion. The returned string is the model's reply (the
// verb's stdout); a non-nil error is the verb failing.
func runVision(ctx context.Context, in *params.VisionInput, paths []string) (string, error) {
	images, err := readImages(paths, in.Mime)
	if err != nil {
		return "", err
	}
	cfg := resolveConfig(in)
	reply, err := llmkit.ChatVision(ctx, cfg, in.Prompt, images)
	if err != nil {
		return "", err
	}
	if err := assertExpect(reply, in.Expect); err != nil {
		// The reply is still returned as the output so the failure message carries
		// the model's actual answer (the evidence a reader needs).
		return reply, err
	}
	return reply, nil
}

// resolveConfig applies the SAME field-wise precedence the pipeline documents:
// built-in default → the step's llm block → the step's params overlay → the
// EVAL_LLM_* operator env. The vision step's own `params:` is a convenience
// overlay onto llm.params (merged, not replaced).
//
// The plugin's OWN params types (params.LLMSpec/params.LLMParams, generated from
// this plugin's served schema) are bridged to the CONTRACT types (spec.LLMSpec/
// spec.LLMParams) the shared SDK client consumes. The two are generated from the
// same CUE vocabulary — the mirror exists so this file compiles STANDALONE — so a
// JSON round-trip is an exact, lossless bridge; toSpec returns the zero value on
// the (impossible) mismatch rather than panicking.
func resolveConfig(in *params.VisionInput) llmkit.Config {
	cfg := llmkit.Default().Apply(toSpec(in.Llm))
	// The params overlay is always applied: MergeParams copies only SET fields, so
	// a zero overlay is a no-op (an omitted `params:` changes nothing).
	cfg = cfg.Apply(spec.LLMSpec{Params: toSpecParams(in.Params)})
	return cfg.FromEnv()
}

// toSpec bridges the plugin's generated #LLMSpec to the contract spec.LLMSpec.
func toSpec(l params.LLMSpec) spec.LLMSpec {
	b, err := json.Marshal(l)
	if err != nil {
		return spec.LLMSpec{}
	}
	var out spec.LLMSpec
	if err := json.Unmarshal(b, &out); err != nil {
		return spec.LLMSpec{}
	}
	return out
}

// toSpecParams bridges the plugin's generated #LLMParams to the contract type.
func toSpecParams(p params.LLMParams) spec.LLMParams {
	b, err := json.Marshal(p)
	if err != nil {
		return spec.LLMParams{}
	}
	var out spec.LLMParams
	if err := json.Unmarshal(b, &out); err != nil {
		return spec.LLMParams{}
	}
	return out
}
