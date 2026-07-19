package token

import (
	"strings"
	"testing"
)

func TestNewDistinct(t *testing.T) {
	p1, _ := New()
	p2, _ := New()
	if p1 == p2 {
		t.Fatalf("New returned identical plaintexts: %q", p1)
	}
}

func TestNewPrefix(t *testing.T) {
	p, _ := New()
	if !strings.HasPrefix(p, "snb_") {
		t.Fatalf("plaintext %q missing snb_ prefix", p)
	}
}

func TestNewHashMatches(t *testing.T) {
	p, h := New()
	if got := Hash(p); got != h {
		t.Fatalf("Hash(plaintext)=%q, want %q", got, h)
	}
}

func TestHashDeterministic(t *testing.T) {
	const in = "snb_example"
	first := Hash(in)
	second := Hash(in)
	if first != second {
		t.Fatalf("Hash not deterministic for %q: %q vs %q", in, first, second)
	}
	if len(first) != 64 {
		t.Fatalf("Hash(%q) length = %d, want 64 hex chars", in, len(first))
	}
}
