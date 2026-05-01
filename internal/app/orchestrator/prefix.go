package orchestrator

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/ui/common"
)

// PrefixMatch describes the result of processing a key in prefix mode.
type PrefixMatch int

const (
	PrefixMatchNone     PrefixMatch = iota // key not recognized, exit prefix mode
	PrefixMatchPartial                     // partial match, stay in prefix mode
	PrefixMatchComplete                    // exact match, execute command
)

// PrefixCommand describes a command reachable via prefix mode.
type PrefixCommand struct {
	Sequence []string
	Label    string
	Help     string
	Action   PrefixAction
}

// PrefixAction is a callback executed when a prefix command matches.
type PrefixAction func() tea.Cmd

// PrefixKeySource provides the prefix key binding.
type PrefixKeySource interface {
	PrefixKeys() []string
}

// PrefixTimeout returns the prefix mode timeout duration.
func PrefixTimeout() time.Duration {
	return 3 * time.Second
}

// PrefixEngine manages the prefix (leader-key) state machine.
// Extracted from app_ui.go.
type PrefixEngine struct {
	Active    bool
	Token     int
	Sequence  []string
	Label     string
	HelpLabel string

	// commands holds the current prefix command palette.
	commands []PrefixCommand
}

// EnterPrefix activates prefix mode and schedules a timeout.
func (pe *PrefixEngine) EnterPrefix() tea.Cmd {
	pe.Active = true
	pe.Sequence = nil
	return pe.RefreshTimeout()
}

// ExitPrefix deactivates prefix mode.
func (pe *PrefixEngine) ExitPrefix() {
	pe.Active = false
	pe.Sequence = nil
}

// OpenPalette opens (or resets) the command palette.
func (pe *PrefixEngine) OpenPalette() tea.Cmd {
	if !pe.Active {
		return pe.EnterPrefix()
	}
	pe.Sequence = nil
	return pe.RefreshTimeout()
}

// RefreshTimeout schedules a new prefix timeout and returns the tick command.
func (pe *PrefixEngine) RefreshTimeout() tea.Cmd {
	return pe.refreshTimeout()
}

// SetCommands replaces the prefix command palette.
func (pe *PrefixEngine) SetCommands(commands []PrefixCommand) {
	pe.commands = commands
}

// Commands returns the current prefix command palette.
func (pe *PrefixEngine) Commands() []PrefixCommand {
	return pe.commands
}

// MatchingCommands returns commands that match the given sequence.
func (pe *PrefixEngine) MatchingCommands(sequence []string) []PrefixCommand {
	return pe.matchingCommands(sequence)
}

// HandleCommand processes a key press while in prefix mode.
// Returns the match result and optional command to execute.
func (pe *PrefixEngine) HandleCommand(msg tea.KeyPressMsg) (PrefixMatch, tea.Cmd) {
	token, ok := pe.inputToken(msg)
	if !ok {
		return PrefixMatchNone, nil
	}

	if token == "backspace" {
		if len(pe.Sequence) > 0 {
			pe.Sequence = pe.Sequence[:len(pe.Sequence)-1]
		}
		return PrefixMatchPartial, nil
	}

	pe.Sequence = append(pe.Sequence, token)

	// Tab selection: single digit 1-9 selects tab.
	if len(pe.Sequence) == 1 {
		if r := []rune(token); len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
			return PrefixMatchComplete, nil // caller handles tab selection
		}
	}

	matches := pe.matchingCommands(pe.Sequence)
	if len(matches) == 0 {
		return PrefixMatchNone, nil
	}

	var exact *PrefixCommand
	exactCount := 0
	for i := range matches {
		if len(matches[i].Sequence) == len(pe.Sequence) {
			exactCount++
			exact = &matches[i]
		}
	}
	if exactCount == 1 && len(matches) == 1 && exact != nil {
		return PrefixMatchComplete, exact.Action()
	}

	return PrefixMatchPartial, nil
}

// SelectedTabIndex returns the 0-based tab index if the current sequence
// is a single digit 1-9, or -1 if not.
func (pe *PrefixEngine) SelectedTabIndex() int {
	if len(pe.Sequence) != 1 {
		return -1
	}
	r := []rune(pe.Sequence[0])
	if len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
		return int(r[0] - '1')
	}
	return -1
}

func (pe *PrefixEngine) inputToken(msg tea.KeyPressMsg) (string, bool) {
	switch msg.Key().Code {
	case tea.KeyBackspace, tea.KeyDelete:
		return "backspace", true
	}
	text := msg.Key().Text
	runes := []rune(text)
	if len(runes) != 1 {
		return "", false
	}
	return text, true
}

func (pe *PrefixEngine) matchingCommands(seq []string) []PrefixCommand {
	var result []PrefixCommand
	for _, cmd := range pe.commands {
		if len(cmd.Sequence) < len(seq) {
			continue
		}
		match := true
		for i, s := range seq {
			if i >= len(cmd.Sequence) || cmd.Sequence[i] != s {
				match = false
				break
			}
		}
		if match {
			result = append(result, cmd)
		}
	}
	return result
}

// refreshTimeout resets the prefix timeout counter and returns a timeout command.
func (pe *PrefixEngine) refreshTimeout() tea.Cmd {
	pe.Token++
	token := pe.Token
	return common.SafeTick(PrefixTimeout(), func(t time.Time) tea.Msg {
		return PrefixTimeoutMsg{Token: token}
	})
}

// PrefixTimeoutMsg is sent when the prefix mode timer expires.
type PrefixTimeoutMsg struct {
	Token int
}
