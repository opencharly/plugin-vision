package vision

import (
	"strings"
	"testing"

	"github.com/opencharly/plugin-vision/candy/plugin-vision/params"
)

// live_test.go — the R10 live proof for the verb. It drives the REAL provider
// Invoke against the REAL local ollama vision endpoint with a REAL generated
// image, proving the end-to-end path: read the PNG → base64 data URL → the shared
// llmkit.ChatVision → the model's answer → the expect: assertion → the verdict.
//
// It SKIPS when no endpoint is reachable (so it never reds a host without
// ollama), but it is the test whose output must be pasted as the verb's proof.
//
// Run: go test -run TestLive -v ./...

// TestLive_VisionAssertionAgainstRealEndpoint: a 64x64 SOLID GREEN image must be
// identified as green — the assertion is on a color the model cannot confuse,
// proving the image really reached the model through the content-parts path.
func TestLive_VisionAssertionAgainstRealEndpoint(t *testing.T) {
	if !endpointUp(t) {
		t.Skip("no OpenAI-compatible endpoint at " + liveBaseURL())
	}
	img := writeSolidPNG(t, 64, 64, 0, 255, 0) // green
	rep := invoke(t, params.VisionInput{
		Prompt:   "Reply with ONLY the dominant color word, nothing else.",
		Artifact: img,
		Expect:   params.VisionExpect{Contains: "green"},
		Params: params.LLMParams{
			Max_tokens: int64ptr(2000),
		},
	}, nil)
	st, msg := replyStatus(t, rep)
	if st != "pass" {
		t.Fatalf("the live model did not identify the green image: status=%q message=%s", st, msg)
	}
	t.Logf("LIVE VERDICT: pass — the model identified the green image; reply=%s", strings.TrimSpace(msg))
}

// TestLive_WrongExpectationFails: the SAME image against a deliberately wrong
// expectation (blue) must FAIL — proving the assertion actually gates the verdict
// rather than the verb passing on any model answer.
func TestLive_WrongExpectationFails(t *testing.T) {
	if !endpointUp(t) {
		t.Skip("no OpenAI-compatible endpoint at " + liveBaseURL())
	}
	img := writeSolidPNG(t, 64, 64, 0, 255, 0) // green
	rep := invoke(t, params.VisionInput{
		Prompt:   "Reply with ONLY the dominant color word, nothing else.",
		Artifact: img,
		Expect:   params.VisionExpect{Contains: "blue"},
		Params: params.LLMParams{
			Max_tokens: int64ptr(2000),
		},
	}, nil)
	st, msg := replyStatus(t, rep)
	if st != "fail" {
		t.Fatalf("a wrong expectation must FAIL, got status=%q message=%s", st, msg)
	}
	t.Logf("LIVE VERDICT: fail (as authored) — the assertion gated the verdict; message=%s", strings.TrimSpace(msg))
}

// endpointUp reports whether the configured OpenAI-compatible endpoint answers.
func endpointUp(t *testing.T) bool {
	t.Helper()
	// reuse the provider's own resolution so the live test dials exactly what the
	// verb would.
	cfg := resolveConfig(&params.VisionInput{})
	if cfg.BaseURL == "" {
		return false
	}
	return probeEndpoint(cfg.BaseURL)
}
