package vision

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencharly/plugin-vision/candy/plugin-vision/params"
	pb "github.com/opencharly/spec/proto"
	"github.com/opencharly/spec/spec"
)

// vision_test.go — the verb's tests. They drive the REAL provider Invoke path
// (decode op + plugin_input → read the image → call the shared client → verdict)
// against a mock OpenAI endpoint, so the whole dispatch is covered end to end.

// mockLLM starts an SSE endpoint and captures the request body.
func mockLLM(t *testing.T, reply func(http.ResponseWriter), assert func(t *testing.T, body map[string]any)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		raw, _ := io.ReadAll(req.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if assert != nil {
			assert(t, body)
		}
		reply(rw)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sseReply(rw http.ResponseWriter, content string) {
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.WriteHeader(http.StatusOK)
	chunk := map[string]any{
		"id": "x", "object": "chat.completion.chunk", "created": 1, "model": "m",
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": content}, "finish_reason": nil}},
	}
	b, _ := json.Marshal(chunk)
	_, _ = rw.Write([]byte("data: " + string(b) + "\n\n"))
	_, _ = rw.Write([]byte("data: [DONE]\n\n"))
}

// writePNG writes a small real PNG file and returns its path.
func writePNG(t *testing.T) string {
	t.Helper()
	const pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	b := mustB64(t, pngB64)
	p := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// invoke drives one provider Invoke with the given input + op modifiers.
func invoke(t *testing.T, in params.VisionInput, op *spec.Op) *pb.InvokeReply {
	t.Helper()
	if op == nil {
		op = &spec.Op{}
	}
	op.PluginInput = toMap(t, in)
	raw, _ := json.Marshal(op)
	rep, err := (provider{}).Invoke(t.Context(), &pb.InvokeRequest{
		ParamsJson: raw,
		EnvJson:    []byte(`{"box":"b","mode":"live"}`),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	return rep
}

// replyStatus decodes the wire {status,message}.
func replyStatus(t *testing.T, rep *pb.InvokeReply) (string, string) {
	t.Helper()
	var v struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rep.ResultJson, &v); err != nil {
		t.Fatalf("decode reply %s: %v", rep.ResultJson, err)
	}
	return v.Status, v.Message
}

// TestInvoke_PromptAndImageReachTheModel: the whole path — the image file is read
// and sent as a base64 data URL, the prompt arrives, and a passing reply yields
// status pass.
func TestInvoke_PromptAndImageReachTheModel(t *testing.T) {
	srv := mockLLM(t, func(rw http.ResponseWriter) { sseReply(rw, "the screenshot shows a red square") },
		func(t *testing.T, body map[string]any) {
			msgs, _ := body["messages"].([]any)
			if len(msgs) == 0 {
				t.Fatal("no messages")
			}
			user, _ := msgs[0].(map[string]any)
			parts, ok := user["content"].([]any)
			if !ok {
				t.Fatalf("content must be a parts array, got %T", user["content"])
			}
			var sawText, sawImage bool
			for _, p := range parts {
				pm, _ := p.(map[string]any)
				switch pm["type"] {
				case "text":
					sawText = true
					if s, _ := pm["text"].(string); !strings.Contains(s, "what color") {
						t.Errorf("prompt not passed: %q", s)
					}
				case "image_url":
					sawImage = true
					iu, _ := pm["image_url"].(map[string]any)
					if url, _ := iu["url"].(string); !strings.HasPrefix(url, "data:image/png;base64,") {
						t.Errorf("image must be a base64 data URL, got %q", url)
					}
				}
			}
			if !sawText || !sawImage {
				t.Errorf("parts need text AND image (text=%v image=%v)", sawText, sawImage)
			}
		})
	t.Setenv("EVAL_LLM_BASE_URL", srv.URL)
	rep := invoke(t, params.VisionInput{Prompt: "what color is this?", Artifact: writePNG(t)}, nil)
	st, msg := replyStatus(t, rep)
	if st != "pass" {
		t.Fatalf("status = %q (%s), want pass", st, msg)
	}
}

// TestInvoke_ExpectContainsPassesAndFails: the `expect:` assertion gates the
// verdict — a matching reply passes, a non-matching one FAILS with the model's
// actual answer in the message (the point of the assertion is a legible failure).
func TestInvoke_ExpectContainsPassesAndFails(t *testing.T) {
	srv := mockLLM(t, func(rw http.ResponseWriter) { sseReply(rw, "the screenshot shows a red square") }, nil)
	t.Setenv("EVAL_LLM_BASE_URL", srv.URL)

	rep := invoke(t, params.VisionInput{Prompt: "color?", Artifact: writePNG(t), Expect: params.VisionExpect{Contains: "red"}}, nil)
	if st, msg := replyStatus(t, rep); st != "pass" {
		t.Fatalf("a matching contains must pass, got %q (%s)", st, msg)
	}

	rep = invoke(t, params.VisionInput{Prompt: "color?", Artifact: writePNG(t), Expect: params.VisionExpect{Contains: "blue"}}, nil)
	st, msg := replyStatus(t, rep)
	if st != "fail" {
		t.Fatalf("a non-matching contains must fail, got %q", st)
	}
	if !strings.Contains(msg, "red square") {
		t.Errorf("the failure must carry the model's actual reply, got %q", msg)
	}
}

// TestInvoke_ExpectMatchesAndNotContains: the regex + negative assertions.
func TestInvoke_ExpectMatchesAndNotContains(t *testing.T) {
	srv := mockLLM(t, func(rw http.ResponseWriter) { sseReply(rw, "the screenshot shows a red square") }, nil)
	t.Setenv("EVAL_LLM_BASE_URL", srv.URL)

	if st, msg := replyStatus(t, invoke(t, params.VisionInput{Prompt: "c", Artifact: writePNG(t), Expect: params.VisionExpect{Matches: `red\s+square`}}, nil)); st != "pass" {
		t.Fatalf("a matching regex must pass, got %q (%s)", st, msg)
	}
	if st, _ := replyStatus(t, invoke(t, params.VisionInput{Prompt: "c", Artifact: writePNG(t), Expect: params.VisionExpect{Not_contains: "red"}}, nil)); st != "fail" {
		t.Fatalf("a forbidden substring must fail, got %q", st)
	}
}

// TestInvoke_PromptRequired: an empty prompt is a hard FAIL (not a silent pass).
func TestInvoke_PromptRequired(t *testing.T) {
	rep := invoke(t, params.VisionInput{Artifact: writePNG(t)}, nil)
	if st, msg := replyStatus(t, rep); st != "fail" || !strings.Contains(msg, "prompt") {
		t.Fatalf("empty prompt must fail naming it, got %q (%s)", st, msg)
	}
}

// TestInvoke_ArtifactRequired: no image is a hard FAIL.
func TestInvoke_ArtifactRequired(t *testing.T) {
	rep := invoke(t, params.VisionInput{Prompt: "what?"}, nil)
	if st, msg := replyStatus(t, rep); st != "fail" || !strings.Contains(msg, "artifact") {
		t.Fatalf("no image must fail naming it, got %q (%s)", st, msg)
	}
}

// TestInvoke_MissingImageFileFails: a bad path is an honest failure.
func TestInvoke_MissingImageFileFails(t *testing.T) {
	rep := invoke(t, params.VisionInput{Prompt: "what?", Artifact: filepath.Join(t.TempDir(), "absent.png")}, nil)
	if st, msg := replyStatus(t, rep); st != "fail" || !strings.Contains(msg, "reading image") {
		t.Fatalf("a missing image must fail informatively, got %q (%s)", st, msg)
	}
}

// TestInvoke_StdoutMatcherGradesTheReply: the shared #Op stdout matcher grades
// the model's reply (the reply IS the verb's output), so a `check:` step's
// stdout: matcher works exactly like on any other verb.
func TestInvoke_StdoutMatcherGradesTheReply(t *testing.T) {
	srv := mockLLM(t, func(rw http.ResponseWriter) { sseReply(rw, "the screenshot shows a red square") }, nil)
	t.Setenv("EVAL_LLM_BASE_URL", srv.URL)

	// a matcher the reply satisfies -> pass
	pass := invoke(t, params.VisionInput{Prompt: "q", Artifact: writePNG(t)}, &spec.Op{
		Stdout: spec.MatcherList{{Op: "contains", Value: "red"}},
	})
	if st, msg := replyStatus(t, pass); st != "pass" {
		t.Fatalf("a satisfied stdout matcher must pass, got %q (%s)", st, msg)
	}
	// a matcher it cannot satisfy -> fail
	fail := invoke(t, params.VisionInput{Prompt: "q", Artifact: writePNG(t)}, &spec.Op{
		Stdout: spec.MatcherList{{Op: "contains", Value: "green"}},
	})
	if st, _ := replyStatus(t, fail); st != "fail" {
		t.Fatalf("an unsatisfied stdout matcher must fail, got %q", st)
	}
}

// TestDetectMimeAndReadImages: the mime inference + the data-URL rendering, and
// the multi-image form (artifact + images).
func TestDetectMimeAndReadImages(t *testing.T) {
	if got := detectMime("a.JPG"); got != "image/jpeg" {
		t.Errorf("jpg -> %q", got)
	}
	if got := detectMime("a.webp"); got != "image/webp" {
		t.Errorf("webp -> %q", got)
	}
	if got := detectMime("a.unknown"); got != "image/png" {
		t.Errorf("unknown -> %q (default png)", got)
	}
	p1, p2 := writePNG(t), writePNG(t)
	imgs, err := readImages([]string{p1, p2}, "")
	if err != nil || len(imgs) != 2 {
		t.Fatalf("readImages: %v len=%d", err, len(imgs))
	}
	for _, u := range imgs {
		if !strings.HasPrefix(u, "data:image/png;base64,") {
			t.Errorf("not a data URL: %q", u)
		}
	}
	// empty file is an honest failure
	empty := filepath.Join(t.TempDir(), "e.png")
	_ = os.WriteFile(empty, nil, 0o644)
	if _, err := readImages([]string{empty}, ""); err == nil {
		t.Error("an empty image file must fail")
	}
}

// TestImagePathsDedupes: imagePaths puts artifact first, appends images, skips
// blanks and de-duplicates.
func TestImagePathsDedupes(t *testing.T) {
	got := imagePaths(&params.VisionInput{Artifact: "a.png", Images: []string{"b.png", "", "a.png"}})
	if strings.Join(got, ",") != "a.png,b.png" {
		t.Fatalf("imagePaths = %v, want [a.png b.png]", got)
	}
}

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("bad base64: %v", err)
	}
	return b
}

// toMap renders a typed input into the desugared plugin_input map (what the host
// ships in the op).
func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
