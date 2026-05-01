package app

import (
	"charm.land/bubbles/v2/key"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/orchestrator"
	"github.com/andyrewlee/amux/internal/messages"
)

// --- Focus management (delegated to orchestrator.FocusManager) ---

// setFocusedPane updates pane focus state without triggering side effects.
func (a *App) setFocusedPane(pane messages.PaneType) {
	a.oc().Focus.SetFocusedPane(pane)
	a.syncPaneFocusFlags()
}

// focusPane changes focus to the specified pane.
func (a *App) focusPane(pane messages.PaneType) tea.Cmd {
	cmd := a.oc().Focus.FocusPane(pane, a)
	a.syncPaneFocusFlags()
	return cmd
}

// focusPaneOnWheel updates focus for hover-wheel routing.
func (a *App) focusPaneOnWheel(pane messages.PaneType) tea.Cmd {
	cmd := a.oc().Focus.FocusPaneOnWheel(pane, a)
	a.syncPaneFocusFlags()
	return cmd
}

// focusPaneLeft moves focus one pane to the left.
func (a *App) focusPaneLeft() tea.Cmd {
	if a.ui == nil || a.ui.layout == nil {
		// Partial App (tests): fallback to simple navigation
		if a.oc().Focus.FocusedPane == messages.PaneCenter {
			return a.focusPane(messages.PaneDashboard)
		}
		return nil
	}
	return a.oc().Focus.FocusLeft(a)
}

// focusPaneRight moves focus one pane to the right.
func (a *App) focusPaneRight() tea.Cmd {
	if a.ui == nil || a.ui.layout == nil {
		// Partial App (tests): fallback to simple navigation
		switch a.oc().Focus.FocusedPane {
		case messages.PaneDashboard:
			return a.focusPane(messages.PaneCenter)
		case messages.PaneCenter:
			return a.focusPane(messages.PaneSidebar)
		}
		return nil
	}
	return a.oc().Focus.FocusRight(a)
}

// Implement orchestrator.FocusContext for App.
func (a *App) Center() orchestrator.CenterTarget {
	if a.ui != nil {
		return a.ui.center
	}
	return nil
}

func (a *App) SidebarTerminal() orchestrator.SidebarTerminalTarget {
	if a.ui != nil {
		return a.ui.sidebarTerminal
	}
	return nil
}

func (a *App) Layout() orchestrator.LayoutInfo {
	if a.ui != nil && a.ui.layout != nil {
		return a.ui.layout
	}
	return nilLayout{}
}

// nilLayout is a fallback LayoutInfo that hides all panes.
type nilLayout struct{}

func (nilLayout) ShowCenter() bool    { return false }
func (nilLayout) ShowDashboard() bool { return false }
func (nilLayout) ShowSidebar() bool   { return false }

// --- Prefix mode (delegated to orchestrator.PrefixEngine) ---

// isPrefixKey returns true if the key is the prefix key.
func (a *App) isPrefixKey(msg tea.KeyPressMsg) bool {
	return key.Matches(msg, a.keymap.Prefix)
}

// enterPrefix enters prefix mode and schedules a timeout.
func (a *App) enterPrefix() tea.Cmd {
	return a.oc().Prefix.EnterPrefix()
}

// openCommandsPalette opens (or resets) the command palette.
func (a *App) openCommandsPalette() tea.Cmd {
	return a.oc().Prefix.OpenPalette()
}

// exitPrefix exits prefix mode.
func (a *App) exitPrefix() {
	a.oc().Prefix.ExitPrefix()
}

// handlePrefixCommand processes a key press while in prefix mode.
// Uses the orchestrator for state management but app's own command matching.
func (a *App) handlePrefixCommand(msg tea.KeyPressMsg) (orchestrator.PrefixMatch, tea.Cmd) {
	token, ok := a.prefixInputToken(msg)
	if !ok {
		return orchestrator.PrefixMatchNone, nil
	}

	if token == "backspace" {
		if len(a.oc().Prefix.Sequence) > 0 {
			a.oc().Prefix.Sequence = a.oc().Prefix.Sequence[:len(a.oc().Prefix.Sequence)-1]
		}
		return orchestrator.PrefixMatchPartial, nil
	}

	a.oc().Prefix.Sequence = append(a.oc().Prefix.Sequence, token)

	if len(a.oc().Prefix.Sequence) == 1 {
		if r := []rune(token); len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
			return orchestrator.PrefixMatchComplete, a.prefixSelectTab(int(r[0] - '1'))
		}
	}

	matches := a.matchingPrefixCommands(a.oc().Prefix.Sequence)
	if len(matches) == 0 {
		return orchestrator.PrefixMatchNone, nil
	}

	var exact *prefixCommandAction
	exactCount := 0
	for i := range matches {
		if len(matches[i].Sequence) == len(a.oc().Prefix.Sequence) {
			exactCount++
			exact = &matches[i]
		}
	}
	if exactCount == 1 && len(matches) == 1 && exact != nil {
		return orchestrator.PrefixMatchComplete, a.runPrefixAction(exact.Action)
	}

	return orchestrator.PrefixMatchPartial, nil
}

func (a *App) prefixInputToken(msg tea.KeyPressMsg) (string, bool) {
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

// --- Remaining pane/tab management (still on App, using orchestrator fields) ---

// cycleTab handles next/prev tab for the focused pane.
func (a *App) cycleTab(sidebarFn, sidebarTermFn func(), centerFn func() tea.Cmd) tea.Cmd {
	switch a.oc().Focus.FocusedPane {
	case messages.PaneSidebarTerminal:
		sidebarTermFn()
	case messages.PaneSidebar:
		sidebarFn()
	default:
		_, before := a.ui.center.GetTabsInfo()
		centerFn()
		_, after := a.ui.center.GetTabsInfo()
		if after == before {
			return nil
		}
		return a.persistActiveWorkspaceTabs()
	}
	return nil
}

// dispatchTabAction dispatches a tab action to center or sidebar terminal.
func (a *App) dispatchTabAction(centerFn, sidebarTermFn func() tea.Cmd) tea.Cmd {
	switch a.oc().Focus.FocusedPane {
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
	return a.ui.center.SelectTab(index)
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
	switch a.oc().Focus.FocusedPane {
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
	switch a.oc().Focus.FocusedPane {
	case messages.PaneCenter:
		a.ui.center.SendToTerminal(raw)
	case messages.PaneSidebarTerminal:
		a.ui.sidebarTerminal.SendToTerminal(raw)
	}
}

// updateLayout updates component sizes based on window size.
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

	sidebarWidth := a.ui.layout.SidebarWidth()
	sidebarHeight := a.ui.layout.Height()
	topPaneHeight, bottomPaneHeight := sidebarPaneHeights(sidebarHeight)
	contentWidth := max(sidebarWidth-2-2, 1)
	topContentHeight := max(topPaneHeight-2, 1)
	bottomContentHeight := max(bottomPaneHeight-2, 1)

	a.ui.sidebar.SetSize(contentWidth, topContentHeight)
	a.ui.sidebarTerminal.SetSize(contentWidth, bottomContentHeight)

	sidebarX := leftGutter + a.ui.layout.DashboardWidth()
	if a.ui.layout.ShowDashboard() && a.ui.layout.ShowCenter() {
		sidebarX += a.ui.layout.GapX()
	}
	if a.ui.layout.ShowCenter() {
		sidebarX += a.ui.layout.CenterWidth()
	}
	sidebarX += a.ui.layout.GapX()
	sidebarContentOffsetX := sidebarX + 2
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
