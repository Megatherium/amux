package app

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// setFocusedPane updates pane focus state without triggering pane-specific side effects.
func (a *App) setFocusedPane(pane messages.PaneType) {
	a.focusedPane = pane
	// Keep focus transitions fail-safe for partially initialized App instances
	// used in lightweight tests.
	a.syncPaneFocusFlags()
}

// focusPane changes focus to the specified pane
func (a *App) focusPane(pane messages.PaneType) tea.Cmd {
	a.setFocusedPane(pane)
	switch pane {
	case messages.PaneCenter:
		// Seamless UX: when center regains focus, attempt reattach for detached active tab.
		if a.ui.center != nil {
			return a.ui.center.ReattachActiveTabIfDetached()
		}
	case messages.PaneSidebarTerminal:
		// Lazy initialization: create terminal on focus if none exists.
		if a.ui.sidebarTerminal != nil {
			return a.ui.sidebarTerminal.EnsureTerminalTab()
		}
	}
	return nil
}

// focusPaneOnWheel updates focus for hover-wheel routing and preserves only the
// center-pane detached-tab reattach behavior. It intentionally skips other
// focus-time side effects such as lazy sidebar terminal creation.
func (a *App) focusPaneOnWheel(pane messages.PaneType) tea.Cmd {
	a.setFocusedPane(pane)
	if pane == messages.PaneCenter && a.ui.center != nil {
		return a.ui.center.ReattachActiveTabIfDetached()
	}
	return nil
}

// focusPaneLeft moves focus one pane to the left, respecting layout visibility.
func (a *App) focusPaneLeft() tea.Cmd {
	switch a.focusedPane {
	case messages.PaneSidebarTerminal, messages.PaneSidebar:
		if a.ui.layout != nil && a.ui.layout.ShowCenter() {
			return a.focusPane(messages.PaneCenter)
		}
		return a.focusPane(messages.PaneDashboard)
	case messages.PaneCenter:
		if a.ui == nil || a.ui.layout == nil || a.ui.layout.ShowDashboard() {
			return a.focusPane(messages.PaneDashboard)
		}
	}
	return nil
}

// focusPaneRight moves focus one pane to the right, respecting layout visibility.
func (a *App) focusPaneRight() tea.Cmd {
	switch a.focusedPane {
	case messages.PaneDashboard:
		if a.ui.layout != nil && a.ui.layout.ShowCenter() {
			return a.focusPane(messages.PaneCenter)
		}
		if a.ui.layout != nil && a.ui.layout.ShowSidebar() {
			return a.focusPane(messages.PaneSidebar)
		}
	case messages.PaneCenter:
		if a.ui.layout != nil && a.ui.layout.ShowSidebar() {
			return a.focusPane(messages.PaneSidebar)
		}
	}
	return nil
}

// Prefix mode helpers (leader key)

// isPrefixKey returns true if the key is the prefix key
func (a *App) isPrefixKey(msg tea.KeyPressMsg) bool {
	return key.Matches(msg, a.keymap.Prefix)
}

// enterPrefix enters prefix mode and schedules a timeout
func (a *App) enterPrefix() tea.Cmd {
	a.prefixActive = true
	a.prefixSequence = nil
	return a.refreshPrefixTimeout()
}

// openCommandsPalette opens (or resets) the bottom command palette.
// This message-driven path is used by mouse/toolbar interactions and therefore
// never sends a literal Ctrl-Space (NUL) to terminals.
func (a *App) openCommandsPalette() tea.Cmd {
	if !a.prefixActive {
		return a.enterPrefix()
	}
	a.prefixSequence = nil
	return a.refreshPrefixTimeout()
}

func (a *App) refreshPrefixTimeout() tea.Cmd {
	a.prefixToken++
	token := a.prefixToken
	return common.SafeTick(PrefixTimeout(), func(t time.Time) tea.Msg {
		return prefixTimeoutMsg{token: token}
	})
}

// exitPrefix exits prefix mode
func (a *App) exitPrefix() {
	a.prefixActive = false
	a.prefixSequence = nil
}

// handlePrefixCommand handles a key press while in prefix mode
// Returns (match state, cmd).
func (a *App) handlePrefixCommand(msg tea.KeyPressMsg) (prefixMatch, tea.Cmd) {
	token, ok := a.prefixInputToken(msg)
	if !ok {
		return prefixMatchNone, nil
	}

	if token == "backspace" {
		if len(a.prefixSequence) > 0 {
			a.prefixSequence = a.prefixSequence[:len(a.prefixSequence)-1]
		}
		// Keep the palette open at root so Backspace remains a harmless undo key.
		return prefixMatchPartial, nil
	}

	a.prefixSequence = append(a.prefixSequence, token)
	// Record the typed token before matching so the palette can render the
	// narrowed path immediately; unknown sequences still fall through to
	// prefixMatchNone below and exit prefix mode in handleKeyPress.

	if len(a.prefixSequence) == 1 {
		if r := []rune(token); len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
			return prefixMatchComplete, a.prefixSelectTab(int(r[0] - '1'))
		}
	}

	matches := a.matchingPrefixCommands(a.prefixSequence)
	if len(matches) == 0 {
		return prefixMatchNone, nil
	}

	var exact *prefixCommand
	exactCount := 0
	for i := range matches {
		if len(matches[i].Sequence) == len(a.prefixSequence) {
			exactCount++
			exact = &matches[i]
		}
	}
	// Execute only when the sequence resolves to a unique leaf command.
	// Ambiguous prefixes intentionally stay in narrowing mode.
	if exactCount == 1 && len(matches) == 1 && exact != nil {
		return prefixMatchComplete, a.runPrefixAction(exact.Action)
	}

	return prefixMatchPartial, nil
}

func (a *App) prefixInputToken(msg tea.KeyPressMsg) (string, bool) {
	switch msg.Key().Code {
	case tea.KeyBackspace, tea.KeyDelete:
		// Some terminals report Backspace as KeyDelete; treat both as undo.
		return "backspace", true
	}
	text := msg.Key().Text
	runes := []rune(text)
	if len(runes) != 1 {
		return "", false
	}
	return text, true
}

// cycleTab handles next/prev tab for the focused pane, persisting center tab changes.
func (a *App) cycleTab(sidebarFn, sidebarTermFn func(), centerFn func() tea.Cmd) tea.Cmd {
	switch a.focusedPane {
	case messages.PaneSidebarTerminal:
		sidebarTermFn()
	case messages.PaneSidebar:
		sidebarFn()
	default:
		_, before := a.ui.center.GetTabsInfo()
		cmd := centerFn()
		_, after := a.ui.center.GetTabsInfo()
		if after == before {
			return nil
		}
		return common.SafeBatch(cmd, a.persistActiveWorkspaceTabs())
	}
	return nil
}

// dispatchTabAction dispatches a tab action to center or sidebar terminal.
func (a *App) dispatchTabAction(centerFn, sidebarTermFn func() tea.Cmd) tea.Cmd {
	switch a.focusedPane {
	case messages.PaneCenter:
		return centerFn()
	case messages.PaneSidebarTerminal:
		return sidebarTermFn()
	}
	return nil
}

func (a *App) requireWorkspaceSelection(action string) tea.Cmd {
	if a.activeWorkspace != nil && a.activeProject != nil {
		return nil
	}
	if a.ui.toast != nil {
		return a.ui.toast.ShowWarning("Select a workspace before " + action)
	}
	return nil
}

func (a *App) prefixSelectTab(index int) tea.Cmd {
	tabs, activeIdx := a.ui.center.GetTabsInfo()
	if index < 0 || index >= len(tabs) || index == activeIdx {
		return nil
	}
	cmd := a.ui.center.SelectTab(index)
	return common.SafeBatch(cmd, a.persistActiveWorkspaceTabs())
}

// togglePaneCollapse toggles a pane's collapse state and relocates focus if needed.
func (a *App) togglePaneCollapse(pane string) tea.Cmd {
	if a.ui == nil || a.ui.layout == nil {
		return nil
	}
	switch pane {
	case "both":
		a.ui.layout.ToggleBoth()
	case "dashboard":
		a.ui.layout.ToggleDashboard()
	case "sidebar":
		a.ui.layout.ToggleSidebar()
	default:
		return nil
	}
	a.updateLayout()

	needsRelocate := false
	switch a.focusedPane {
	case messages.PaneDashboard:
		needsRelocate = !a.ui.layout.ShowDashboard()
	case messages.PaneSidebar, messages.PaneSidebarTerminal:
		needsRelocate = !a.ui.layout.ShowSidebar()
	}
	if needsRelocate {
		a.focusPane(messages.PaneCenter)
	} else {
		a.syncPaneFocusFlags()
	}
	return nil
}

// sendPrefixToTerminal sends a literal prefix key byte to the focused terminal.
// For the default C-Space it sends NUL (\x00). For custom overrides like ctrl+p
// it sends the appropriate control byte. For keys with no single-byte form (e.g.
// function keys), it sends nothing.
func (a *App) sendPrefixToTerminal() {
	keys := a.keymap.Prefix.Keys()
	if len(keys) == 0 {
		return
	}
	b := PrefixKeyByte(keys[0])
	if b < 0 {
		return
	}
	raw := string([]byte{byte(b)})
	switch a.focusedPane {
	case messages.PaneCenter:
		a.ui.center.SendToTerminal(raw)
	case messages.PaneSidebarTerminal:
		a.ui.sidebarTerminal.SendToTerminal(raw)
	}
}

// updateLayout updates component sizes based on window size
func (a *App) updateLayout() {
	a.ui.dashboard.SetSize(a.ui.layout.DashboardWidth(), a.ui.layout.Height())

	centerWidth := a.ui.layout.CenterWidth()
	a.ui.center.SetSize(centerWidth, a.ui.layout.Height())
	leftGutter := a.ui.layout.LeftGutter()
	topGutter := a.ui.layout.TopGutter()
	gapX := 0
	if a.ui.layout.ShowCenter() && a.ui.layout.ShowDashboard() {
		gapX = a.ui.layout.GapX()
	}
	a.ui.center.SetOffset(
		leftGutter + a.ui.layout.DashboardWidth() + gapX,
	)
	a.ui.center.SetCanFocusRight(a.ui.layout.ShowSidebar())
	a.ui.dashboard.SetCanFocusRight(a.ui.layout.ShowCenter())

	// New two-pane sidebar structure: each pane has its own border
	sidebarWidth := a.ui.layout.SidebarWidth()
	sidebarHeight := a.ui.layout.Height()

	// Each pane gets half the height (borders touch)
	topPaneHeight, bottomPaneHeight := sidebarPaneHeights(sidebarHeight)

	// Content dimensions inside each pane (subtract border + padding)
	// Border: 2 (top + bottom), Padding: 2 (left + right from Pane style)
	contentWidth := max(
		// border + padding
		sidebarWidth-2-2, 1)
	topContentHeight := max(
		// border only (no vertical padding in Pane style)
		topPaneHeight-2, 1)
	bottomContentHeight := max(bottomPaneHeight-2, 1)

	a.ui.sidebar.SetSize(contentWidth, topContentHeight)
	a.ui.sidebarTerminal.SetSize(contentWidth, bottomContentHeight)

	// Calculate and set offsets for sidebar mouse handling
	// X: Dashboard + Center + Border(1) + Padding(1)
	sidebarX := leftGutter + a.ui.layout.DashboardWidth()
	if a.ui.layout.ShowDashboard() && a.ui.layout.ShowCenter() {
		sidebarX += a.ui.layout.GapX()
	}
	if a.ui.layout.ShowCenter() {
		sidebarX += a.ui.layout.CenterWidth()
	}
	sidebarX += a.ui.layout.GapX()
	sidebarContentOffsetX := sidebarX + 2 // +2 for border and padding

	// Y: Top pane height (including its border) + Bottom pane border(1)
	termOffsetY := topGutter + topPaneHeight + 1
	a.ui.sidebarTerminal.SetOffset(sidebarContentOffsetX, termOffsetY)

	if a.ui.dialog != nil {
		a.ui.dialog.SetSize(a.ui.width, a.ui.height)
	}
	if a.ui.filePicker != nil {
		a.ui.filePicker.SetSize(a.ui.width, a.ui.height)
	}
	if a.ui.settingsDialog != nil {
		a.ui.settingsDialog.SetSize(a.ui.width, a.ui.height)
	}
}

func (a *App) setKeymapHintsEnabled(enabled bool) {
	if a.config != nil {
		a.config.UI.ShowKeymapHints = enabled
	}
	a.ui.dashboard.SetShowKeymapHints(enabled)
	a.ui.center.SetShowKeymapHints(enabled)
	a.ui.sidebar.SetShowKeymapHints(enabled)
	a.ui.sidebarTerminal.SetShowKeymapHints(enabled)
	if a.ui.dialog != nil {
		a.ui.dialog.SetShowKeymapHints(enabled)
	}
	if a.ui.filePicker != nil {
		a.ui.filePicker.SetShowKeymapHints(enabled)
	}
}

func sidebarPaneHeights(total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	top := total / 2
	bottom := total - top

	// Prefer keeping both panes visible when there's room.
	if total >= 6 {
		if top < 3 {
			top = 3
			bottom = total - top
		}
		if bottom < 3 {
			bottom = 3
			top = total - bottom
		}
		return top, bottom
	}

	// In tight spaces, keep the terminal visible by shrinking the top pane first.
	if total >= 3 && bottom < 3 {
		bottom = 3
		top = max(total-bottom, 0)
		return top, bottom
	}

	if top > total {
		top = total
	}
	if bottom < 0 {
		bottom = 0
	}
	return top, bottom
}
