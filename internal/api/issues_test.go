package api

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "photo.jpg", "photo.jpg"},
		{"unix path", "/etc/passwd", "passwd"},
		{"windows path", `C:\Users\a\evil.png`, "evil.png"},
		{"spaces", "my photo.png", "my_photo.png"},
		{"traversal", "../../secret", "secret"},
		{"empty", "", "file"},
		{"dot", ".", "file"},
		{"tabs", "a\tb.txt", "a_b.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitize(tt.in); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
