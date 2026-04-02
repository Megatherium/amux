package tmux

import (
	"strings"
	"testing"
)

func TestAppendSessionTagsWithTicketMetadata(t *testing.T) {
	opts := Options{
		ServerName:      "test-server",
		ConfigPath:      "/dev/null",
		HideStatus:      true,
		DisableMouse:    true,
		DefaultTerminal: "xterm-256color",
	}

	tags := SessionTags{
		WorkspaceID: "ws-1",
		TabID:       "tab-2",
		TicketID:    "bmx-e1u",
		TicketTitle: "Fix the thing",
		Model:       "claude-sonnet-4-20250514",
		AgentMode:   "code",
	}

	cmd := NewClientCommand("test-session", ClientCommandParams{
		WorkDir:        "/tmp/work",
		Command:        "echo hello",
		Options:        opts,
		Tags:           tags,
		DetachExisting: true,
	})

	for _, want := range []string{
		"@amux_ticket_id 'bmx-e1u'",
		"@amux_ticket_title 'Fix the thing'",
		"@amux_model 'claude-sonnet-4-20250514'",
		"@amux_agent_mode 'code'",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("Command should contain %q", want)
		}
	}

	if !strings.Contains(cmd, "@amux_workspace 'ws-1'") {
		t.Error("Existing @amux_workspace tag missing")
	}
}

func TestAppendSessionTagsTicketFieldsEmpty(t *testing.T) {
	opts := Options{
		ServerName:      "test-server",
		ConfigPath:      "/dev/null",
		HideStatus:      true,
		DisableMouse:    true,
		DefaultTerminal: "xterm-256color",
	}

	tags := SessionTags{
		WorkspaceID: "ws-1",
		TabID:       "tab-2",
	}

	cmd := NewClientCommand("test-session", ClientCommandParams{
		WorkDir:        "/tmp/work",
		Command:        "echo hello",
		Options:        opts,
		Tags:           tags,
		DetachExisting: true,
	})

	for _, forbidden := range []string{
		"@amux_ticket_id",
		"@amux_ticket_title",
		"@amux_model",
		"@amux_agent_mode",
	} {
		if strings.Contains(cmd, forbidden) {
			t.Errorf("Command should NOT contain %q when ticket fields are empty", forbidden)
		}
	}

	if !strings.Contains(cmd, "@amux_workspace 'ws-1'") {
		t.Error("Existing @amux_workspace tag missing")
	}
}

func TestAppendSessionTagsTicketTitleShellSpecialChars(t *testing.T) {
	opts := Options{
		ServerName:      "test-server",
		ConfigPath:      "/dev/null",
		HideStatus:      true,
		DisableMouse:    true,
		DefaultTerminal: "xterm-256color",
	}

	tags := SessionTags{
		WorkspaceID: "ws-1",
		TabID:       "tab-2",
		TicketTitle: "Fix O'Brien's $HOME bug: \"it's broken\"",
	}

	cmd := NewClientCommand("test-session", ClientCommandParams{
		WorkDir:        "/tmp/work",
		Command:        "echo hello",
		Options:        opts,
		Tags:           tags,
		DetachExisting: true,
	})

	if !strings.Contains(cmd, "@amux_ticket_title 'Fix O'\\''Brien'\\''s $HOME bug: \"it'\\''s broken\"'") {
		t.Errorf("Command should contain properly shell-quoted ticket title, got: %s", cmd)
	}
}

func TestSanitizeTicketTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "normal", input: "Fix login bug", want: "Fix login bug"},
		{name: "newline replaced", input: "Fix login\nbug", want: "Fix login bug"},
		{name: "carriage return", input: "Title\rhere", want: "Title here"},
		{name: "tab replaced", input: "Fix\tlogin", want: "Fix login"},
		{name: "multiple newlines", input: "Line1\n\nLine2\nLine3", want: "Line1  Line2 Line3"},
		{name: "leading/trailing whitespace", input: "  spaced out  ", want: "spaced out"},
		{name: "mixed whitespace", input: "  \n\t  hello\nworld  \n  ", want: "hello world"},
		{name: "at 200 chars", input: strings.Repeat("a", 200), want: strings.Repeat("a", 200)},
		{name: "over 200 chars", input: strings.Repeat("b", 250), want: strings.Repeat("b", 200)},
		{name: "single char", input: "x", want: "x"},
		{name: "multi-byte UTF-8 truncated", input: "日本語" + strings.Repeat("a", 200), want: "日本語" + strings.Repeat("a", 197)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTicketTitle(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeTicketTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
