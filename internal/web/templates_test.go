package web

import "testing"

func TestShortID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"0192f1a2-b3c4", "0192f1a2"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
	}
	for _, c := range cases {
		if got := shortID(c.in); got != c.want {
			t.Errorf("shortID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDeref(t *testing.T) {
	if got := deref(nil); got != 0 {
		t.Errorf("deref(nil) = %d, want 0", got)
	}
	v := int64(42)
	if got := deref(&v); got != 42 {
		t.Errorf("deref(&42) = %d, want 42", got)
	}
}

func TestParseTemplatesCoversAllPages(t *testing.T) {
	tmpl := parseTemplates()
	for name := range pageFiles {
		if tmpl[name] == nil {
			t.Errorf("parseTemplates missing template %q", name)
		}
	}
}
