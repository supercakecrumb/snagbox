package attachment

import (
	"net/http"
	"strings"
	"testing"
)

func TestUntrustedAttachmentsCannotExecute(t *testing.T) {
	for _, typ := range []string{"text/html", "image/svg+xml", "application/javascript", "", "invalid"} {
		h := make(http.Header)
		SetHeaders(h, typ, "upload.html")
		if h.Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") {
			t.Fatalf("unsafe download policy for %q", typ)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
			t.Fatal("missing execution protection")
		}
	}
	h := make(http.Header)
	SetHeaders(h, "image/png", "photo.png")
	if h.Get("Content-Type") != "image/png" || !strings.HasPrefix(h.Get("Content-Disposition"), "inline;") {
		t.Fatal("raster preview unavailable")
	}
}
