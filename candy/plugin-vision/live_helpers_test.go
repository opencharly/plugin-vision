package vision

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencharly/plugin-vision/candy/plugin-vision/params"
)

// live_helpers_test.go — the live-test helpers (endpoint probe, real image
// generation). Kept separate from the mock test file so the live suite is legible.

// liveBaseURL is the endpoint the verb resolves by default.
func liveBaseURL() string {
	return resolveConfig(&params.VisionInput{}).BaseURL
}

// probeEndpoint reports whether the endpoint answers /models.
func probeEndpoint(base string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// writeSolidPNG writes a w x h solid-RGB PNG and returns its path.
func writeSolidPNG(t *testing.T, w, h int, r, g, b uint8) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding the PNG fixture: %v", err)
	}
	p := filepath.Join(t.TempDir(), "solid.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// int64ptr is the param-pointer helper for the generated *int64 fields.
func int64ptr(v int64) *int64 { return &v }
