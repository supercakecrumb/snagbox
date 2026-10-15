// Package attachment sets safe headers for user-submitted downloads.
package attachment

import (
	"mime"
	"net/http"
)

// SetHeaders permits inline raster photos and downloads all active content.
func SetHeaders(h http.Header, contentType, filename string) {
	mediaType, _, err := mime.ParseMediaType(contentType)
	disposition := "attachment"
	if err == nil {
		switch mediaType {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			disposition = "inline"
		}
	}
	if disposition == "attachment" {
		mediaType = "application/octet-stream"
	}
	h.Set("Content-Type", mediaType)
	h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": filename}))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
}
