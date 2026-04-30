package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"

	"github.com/andyrewlee/amux/internal/logging"
)

const defaultPrefixEnv = ""

// PrefixKeyLabel returns the display label for the current prefix key.
func PrefixKeyLabel() string {
	env := os.Getenv("AMUX_PREFIX_KEY")
	if env == "" {
		return "C-Space"
	}
	_, label := ParsePrefixKey(env)
	return label
}

// ParsePrefixKey parses a comma-separated list of key names and returns the
// key names suitable for key.NewBinding and a short display label.
// On invalid input it returns the default ctrl+@/ctrl+space binding.
func ParsePrefixKey(raw string) ([]string, string) {
	if raw == "" {
		return []string{"ctrl+@", "ctrl+space"}, "C-Space"
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		k := strings.TrimSpace(p)
		if k == "" {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return []string{"ctrl+@", "ctrl+space"}, "C-Space"
	}
	label := keyNameToLabel(keys[0])
	return keys, label
}

// PrefixKeyByte returns the raw byte to send to a terminal for the given key
// name, or -1 if no single-byte representation exists (e.g. function keys).
//
//nolint:cyclop,funlen // legacy suppression
func PrefixKeyByte(keyName string) int {
	lower := strings.ToLower(keyName)
	if strings.HasPrefix(lower, "ctrl+") {
		suffix := lower[len("ctrl+"):]
		switch suffix {
		case "space", "@":
			return 0
		case "a":
			return 1
		case "b":
			return 2
		case "c":
			return 3
		case "d":
			return 4
		case "e":
			return 5
		case "f":
			return 6
		case "g":
			return 7
		case "h":
			return 8
		case "i":
			return 9
		case "j":
			return 10
		case "k":
			return 11
		case "l":
			return 12
		case "m":
			return 13
		case "n":
			return 14
		case "o":
			return 15
		case "p":
			return 16
		case "q":
			return 17
		case "r":
			return 18
		case "s":
			return 19
		case "t":
			return 20
		case "u":
			return 21
		case "v":
			return 22
		case "w":
			return 23
		case "x":
			return 24
		case "y":
			return 25
		case "z":
			return 26
		case "[":
			return 27
		case "\\":
			return 28
		case "]":
			return 29
		case "^":
			return 30
		case "_":
			return 31
		}
	}
	return -1
}

// keyNameToLabel converts a Bubble Tea key name to a short display label.
func keyNameToLabel(name string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "ctrl+") {
		suffix := lower[len("ctrl+"):]
		switch suffix {
		case "space", "@":
			return "C-Space"
		}
		return "C-" + strings.ToUpper(suffix)
	}
	if strings.HasPrefix(lower, "alt+") {
		return "M-" + keyNameToLabel(lower[len("alt+"):])
	}
	if strings.HasPrefix(lower, "f") && len(lower) <= 3 {
		return strings.ToUpper(name)
	}
	return name
}

// PrefixHelpLabel returns the short display label used in help bars.
// Uses "C-Spc" for the default (matches existing convention) and the
// full label for custom overrides.
func PrefixHelpLabel() string {
	label := PrefixKeyLabel()
	if label == "C-Space" {
		return "C-Spc"
	}
	return label
}

// PrefixTimeout returns the prefix mode timeout duration, reading from
// AMUX_PREFIX_TIMEOUT (e.g. "5s", "500ms"). Falls back to
// defaultPrefixTimeout on missing or invalid values.
func PrefixTimeout() time.Duration {
	value := strings.TrimSpace(os.Getenv("AMUX_PREFIX_TIMEOUT"))
	if value == "" {
		return defaultPrefixTimeout
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		logging.Warn("Invalid AMUX_PREFIX_TIMEOUT=%q; using %s", value, defaultPrefixTimeout)
		return defaultPrefixTimeout
	}
	return d
}

// KeyMap defines all keybindings for the application
type KeyMap struct {
	// Prefix key (leader)
	Prefix key.Binding

	// Dashboard
	Enter        key.Binding
	Delete       key.Binding
	ToggleFilter key.Binding
	Refresh      key.Binding

	// Agent/Chat
	Interrupt  key.Binding
	SendEscape key.Binding

	// Navigation
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return DefaultKeyMapWithPrefix(defaultPrefixEnv)
}

// buildKeymapFromEnv reads AMUX_PREFIX_KEY and builds the keymap.
func buildKeymapFromEnv() KeyMap {
	return DefaultKeyMapWithPrefix(os.Getenv("AMUX_PREFIX_KEY"))
}

// DefaultKeyMapWithPrefix builds the keymap with a custom prefix key.
// prefixKeyEnv is a comma-separated list of key names (e.g. "ctrl+p" or
// "ctrl+@,ctrl+space"). An empty string uses the default prefix.
func DefaultKeyMapWithPrefix(prefixKeyEnv string) KeyMap {
	keys, label := ParsePrefixKey(prefixKeyEnv)

	return KeyMap{
		Prefix: key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(label, "Commands"),
		),

		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "activate"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		ToggleFilter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("g", "r"),
			key.WithHelp("g", "rescan"),
		),

		Interrupt: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "interrupt"),
		),
		SendEscape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "escape"),
		),

		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/down", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/left", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/right", "right"),
		),
	}
}

// ValidatePrefixKey returns a non-nil error if the key names in raw are
// syntactically invalid (e.g. empty after trimming).
func ValidatePrefixKey(raw string) error {
	keys, _ := ParsePrefixKey(raw)
	if len(keys) == 0 {
		return fmt.Errorf("no valid key names in %q", raw)
	}
	return nil
}
