// Package vision is the charly plugin serving the `vision` check verb (an
// importable root package + its own go.mod). It validates an IMAGE — a screenshot
// a check just captured — by sending it DIRECTLY to an OpenAI-compatible vision
// endpoint from charly, and asserts the model's answer.
//
// WHY A CHECK VERB. A screenshot produced by `wl:`/`vnc:`/`cdp:`/`spice:`
// screenshot is a file on the host; the question "does this screenshot show the
// new layout / the right color / the expected state" is not a pixel assertion a
// shell command can express. This verb reads that file, sends it as a base64 data
// URL (ollama's OpenAI layer accepts base64 data URLs ONLY — remote image URLs are
// unsupported), and grades the reply with the shared matcher/verdict pipeline, so
// a `check:` step can assert on an image exactly like any other verb.
//
// DUAL PLACEMENT. The SAME NewProvider()/NewMeta() compile INTO charly in-process
// for a build that lists this plugin in compiled_plugins, or cmd/serve serves them
// OUT-OF-PROCESS over go-plugin gRPC — placement is invisible above the registry.
//
// The OpenAI client is the SDK's shared llmkit (R3): the SAME client the pipeline
// engine's agent stages use, so both speak the endpoint identically (idle bound,
// no env bleed, reasoning capture, vision content parts).
package vision

import (
	"embed"

	"github.com/opencharly/sdk"
	pb "github.com/opencharly/spec/proto"
)

//go:embed schema/*.cue
var schemaFS embed.FS

// NewProvider returns the vision provider.
func NewProvider() pb.ProviderServer {
	return &provider{}
}

// NewMeta advertises verb:vision + the plugin's self-contained CUE schema. The
// verb's entire authoring contract — the image source, the prompt, the endpoint
// config and the reply assertion — lives in the served #VisionInput
// (schema/vision.cue), which the host splices onto the base and validates every
// authored `vision:` step against.
func NewMeta() pb.PluginMetaServer {
	return sdk.NewMeta("2026.259.2200",
		[]sdk.ProvidedCapability{{Class: "verb", Word: "vision", InputDef: "#VisionInput"}},
		schemaFS)
}
