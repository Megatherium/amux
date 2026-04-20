package app

import (
	"os"
	"testing"
	"time"
)

func TestParsePrefixKeyDefault(t *testing.T) {
	keys, label := ParsePrefixKey("")
	if len(keys) != 2 || keys[0] != "ctrl+@" || keys[1] != "ctrl+space" {
		t.Fatalf("expected default keys, got %v", keys)
	}
	if label != "C-Space" {
		t.Fatalf("expected C-Space, got %s", label)
	}
}

func TestParsePrefixKeyCtrlP(t *testing.T) {
	keys, label := ParsePrefixKey("ctrl+p")
	if len(keys) != 1 || keys[0] != "ctrl+p" {
		t.Fatalf("expected [ctrl+p], got %v", keys)
	}
	if label != "C-P" {
		t.Fatalf("expected C-P, got %s", label)
	}
}

func TestParsePrefixKeyMultiBinding(t *testing.T) {
	keys, label := ParsePrefixKey("ctrl+@,ctrl+space")
	if len(keys) != 2 || keys[0] != "ctrl+@" || keys[1] != "ctrl+space" {
		t.Fatalf("expected [ctrl+@ ctrl+space], got %v", keys)
	}
	if label != "C-Space" {
		t.Fatalf("expected C-Space, got %s", label)
	}
}

func TestParsePrefixKeyWhitespace(t *testing.T) {
	keys, label := ParsePrefixKey("  ctrl+p , ctrl+space  ")
	if len(keys) != 2 || keys[0] != "ctrl+p" || keys[1] != "ctrl+space" {
		t.Fatalf("expected trimmed keys, got %v", keys)
	}
	if label != "C-P" {
		t.Fatalf("expected C-P, got %s", label)
	}
}

func TestParsePrefixKeyEmptyCommas(t *testing.T) {
	keys, _ := ParsePrefixKey(",,ctrl+p,,")
	if len(keys) != 1 || keys[0] != "ctrl+p" {
		t.Fatalf("expected [ctrl+p], got %v", keys)
	}
}

func TestParsePrefixKeyAllEmpty(t *testing.T) {
	keys, label := ParsePrefixKey(",,,")
	if len(keys) != 2 {
		t.Fatalf("expected default keys for all-empty input, got %v", keys)
	}
	if label != "C-Space" {
		t.Fatalf("expected C-Space fallback, got %s", label)
	}
}

func TestKeyNameToLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ctrl+space", "C-Space"},
		{"ctrl+@", "C-Space"},
		{"ctrl+p", "C-P"},
		{"ctrl+a", "C-A"},
		{"f1", "F1"},
		{"f12", "F12"},
		{"alt+p", "M-p"},
	}
	for _, tt := range tests {
		got := keyNameToLabel(tt.input)
		if got != tt.want {
			t.Errorf("keyNameToLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPrefixKeyByte(t *testing.T) {
	tests := []struct {
		key  string
		want int
	}{
		{"ctrl+space", 0},
		{"ctrl+@", 0},
		{"ctrl+p", 16},
		{"ctrl+a", 1},
		{"ctrl+z", 26},
		{"ctrl+[", 27},
		{"ctrl+\\", 28},
		{"ctrl+]", 29},
		{"f1", -1},
		{"alt+p", -1},
		{"a", -1},
	}
	for _, tt := range tests {
		got := PrefixKeyByte(tt.key)
		if got != tt.want {
			t.Errorf("PrefixKeyByte(%q) = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestDefaultKeyMapDefault(t *testing.T) {
	km := DefaultKeyMap()
	keys := km.Prefix.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 default keys, got %v", keys)
	}
}

func TestDefaultKeyMapWithPrefixOverride(t *testing.T) {
	km := DefaultKeyMapWithPrefix("ctrl+p")
	keys := km.Prefix.Keys()
	if len(keys) != 1 || keys[0] != "ctrl+p" {
		t.Fatalf("expected [ctrl+p], got %v", keys)
	}
	help := km.Prefix.Help()
	if help.Key != "C-P" {
		t.Fatalf("expected help key C-P, got %s", help.Key)
	}
}

func TestDefaultKeyMapWithPrefixEmpty(t *testing.T) {
	km := DefaultKeyMapWithPrefix("")
	keys := km.Prefix.Keys()
	if len(keys) != 2 || keys[0] != "ctrl+@" || keys[1] != "ctrl+space" {
		t.Fatalf("expected default keys, got %v", keys)
	}
}

func TestPrefixKeyLabelDefault(t *testing.T) {
	os.Setenv("AMUX_PREFIX_KEY", "")
	defer os.Unsetenv("AMUX_PREFIX_KEY")
	if got := PrefixKeyLabel(); got != "C-Space" {
		t.Fatalf("expected C-Space, got %s", got)
	}
}

func TestPrefixHelpLabelDefault(t *testing.T) {
	os.Setenv("AMUX_PREFIX_KEY", "")
	defer os.Unsetenv("AMUX_PREFIX_KEY")
	if got := PrefixHelpLabel(); got != "C-Spc" {
		t.Fatalf("expected C-Spc, got %s", got)
	}
}

func TestPrefixKeyLabelOverride(t *testing.T) {
	os.Setenv("AMUX_PREFIX_KEY", "ctrl+p")
	defer os.Unsetenv("AMUX_PREFIX_KEY")
	if got := PrefixKeyLabel(); got != "C-P" {
		t.Fatalf("expected C-P, got %s", got)
	}
	if got := PrefixHelpLabel(); got != "C-P" {
		t.Fatalf("expected C-P for help label too, got %s", got)
	}
}

func TestValidatePrefixKey(t *testing.T) {
	if err := ValidatePrefixKey("ctrl+p"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidatePrefixKey("ctrl+p,ctrl+space"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidatePrefixKey(""); err != nil {
		t.Fatalf("empty string should be valid (uses default): %v", err)
	}
	if err := ValidatePrefixKey(",,,"); err != nil {
		t.Fatalf("expected error for all-empty keys, got nil")
	}
}

func TestBuildKeymapFromEnv(t *testing.T) {
	os.Setenv("AMUX_PREFIX_KEY", "ctrl+p")
	defer os.Unsetenv("AMUX_PREFIX_KEY")
	km := buildKeymapFromEnv()
	keys := km.Prefix.Keys()
	if len(keys) != 1 || keys[0] != "ctrl+p" {
		t.Fatalf("expected [ctrl+p] from env, got %v", keys)
	}
}

func TestPrefixTimeoutDefault(t *testing.T) {
	os.Setenv("AMUX_PREFIX_TIMEOUT", "")
	defer os.Unsetenv("AMUX_PREFIX_TIMEOUT")
	if got := PrefixTimeout(); got != defaultPrefixTimeout {
		t.Fatalf("expected default %s, got %s", defaultPrefixTimeout, got)
	}
}

func TestPrefixTimeoutOverride(t *testing.T) {
	os.Setenv("AMUX_PREFIX_TIMEOUT", "10s")
	defer os.Unsetenv("AMUX_PREFIX_TIMEOUT")
	if got := PrefixTimeout(); got != 10*time.Second {
		t.Fatalf("expected 10s, got %s", got)
	}
}

func TestPrefixTimeoutMilliseconds(t *testing.T) {
	os.Setenv("AMUX_PREFIX_TIMEOUT", "500ms")
	defer os.Unsetenv("AMUX_PREFIX_TIMEOUT")
	if got := PrefixTimeout(); got != 500*time.Millisecond {
		t.Fatalf("expected 500ms, got %s", got)
	}
}

func TestPrefixTimeoutInvalidFallsBack(t *testing.T) {
	os.Setenv("AMUX_PREFIX_TIMEOUT", "not-a-duration")
	defer os.Unsetenv("AMUX_PREFIX_TIMEOUT")
	if got := PrefixTimeout(); got != defaultPrefixTimeout {
		t.Fatalf("expected default fallback for invalid value, got %s", got)
	}
}

func TestPrefixTimeoutNegativeFallsBack(t *testing.T) {
	os.Setenv("AMUX_PREFIX_TIMEOUT", "-5s")
	defer os.Unsetenv("AMUX_PREFIX_TIMEOUT")
	if got := PrefixTimeout(); got != defaultPrefixTimeout {
		t.Fatalf("expected default fallback for negative value, got %s", got)
	}
}

func TestPrefixTimeoutZeroFallsBack(t *testing.T) {
	os.Setenv("AMUX_PREFIX_TIMEOUT", "0s")
	defer os.Unsetenv("AMUX_PREFIX_TIMEOUT")
	if got := PrefixTimeout(); got != defaultPrefixTimeout {
		t.Fatalf("expected default fallback for zero value, got %s", got)
	}
}
