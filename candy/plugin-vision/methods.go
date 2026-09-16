package vision

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/opencharly/plugin-vision/candy/plugin-vision/params"
	"github.com/opencharly/sdk/llmkit"
)

// methods.go — the verb's image read + reply assertion. The model call itself is
// the SDK's shared llmkit.ChatVision (R3); this file owns only what is specific to
// the verb.

// readImages reads each image file and renders it as an OpenAI image_url data URL.
//
// Ollama's OpenAI layer accepts a BASE64 DATA URL ONLY (the docs mark
// `[ ] Image URL` unsupported), so a remote URL could never reach the model
// through this endpoint — reading the bytes here is the only correct form.
func readImages(paths []string, mime string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading image %s: %w", p, err)
		}
		if len(b) == 0 {
			return nil, fmt.Errorf("image %s is empty", p)
		}
		m := mime
		if m == "" {
			m = detectMime(p)
		}
		out = append(out, llmkit.ImageDataURL(m, b))
	}
	return out, nil
}

// detectMime infers the data-URL mime from the file name; it defaults to
// image/png (the screenshot verbs all write PNG).
func detectMime(path string) string {
	p := strings.ToLower(path)
	switch {
	case strings.HasSuffix(p, ".jpg"), strings.HasSuffix(p, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(p, ".webp"):
		return "image/webp"
	case strings.HasSuffix(p, ".gif"):
		return "image/gif"
	default:
		return "image/png"
	}
}

// assertExpect evaluates the `expect:` reply assertion. Every declared field must
// hold. A failure names the field and what the model actually said — the whole
// point of the assertion is to make a wrong screenshot legible.
func assertExpect(reply string, e params.VisionExpect) error {
	if e.Contains != "" && !strings.Contains(strings.ToLower(reply), strings.ToLower(e.Contains)) {
		return fmt.Errorf("vision assertion failed: the reply does not contain %q (reply: %s)", e.Contains, preview(reply))
	}
	if e.Not_contains != "" && strings.Contains(strings.ToLower(reply), strings.ToLower(e.Not_contains)) {
		return fmt.Errorf("vision assertion failed: the reply contains the forbidden %q (reply: %s)", e.Not_contains, preview(reply))
	}
	if e.Matches != "" {
		re, err := regexp.Compile(e.Matches)
		if err != nil {
			return fmt.Errorf("vision assertion: invalid `matches` regex %q: %w", e.Matches, err)
		}
		if !re.MatchString(reply) {
			return fmt.Errorf("vision assertion failed: the reply does not match %q (reply: %s)", e.Matches, preview(reply))
		}
	}
	return nil
}

// preview bounds a reply in a diagnostic so a long generation cannot flood a log.
func preview(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= 300 {
		return s
	}
	return s[:300] + "…"
}
