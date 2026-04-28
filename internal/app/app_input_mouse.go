package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/common"
)

const paneNone messages.PaneType = -1

// routeMouseClick routes mouse click events to the appropriate pane.
func (a *App) routeMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if a.prefixPaletteContainsPoint(msg.X, msg.Y) {
		// Palette clicks are currently non-interactive; consume to prevent
		// accidental clicks in underlying panes while prefix mode is active.
		return nil
	}

	targetPane, hasTarget := a.paneForPoint(msg.X, msg.Y)

	// Left-click updates keyboard focus; other buttons preserve keyboard focus.
	var focusCmd tea.Cmd
	if msg.Button == tea.MouseLeft && hasTarget {
		focusCmd = a.focusPane(targetPane)
	}

	if cmd := a.handleCenterPaneClick(msg); cmd != nil {
		return common.SafeBatch(focusCmd, cmd)
	}

	// Intentional pointer-target routing (not focused-pane routing): clicks go to
	// the pane under the pointer, including right/middle buttons.
	if !hasTarget {
		return focusCmd
	}

	switch targetPane {
	case messages.PaneDashboard:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X -= a.ui.layout.LeftGutter()
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newDashboard, cmd := a.ui.dashboard.Update(adjusted)
		a.ui.dashboard = newDashboard
		return common.SafeBatch(focusCmd, cmd)
	case messages.PaneCenter:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newCenter, cmd := a.ui.center.Update(adjusted)
		a.ui.center = newCenter
		return common.SafeBatch(focusCmd, cmd)
	case messages.PaneSidebarTerminal:
		newTerm, cmd := a.ui.sidebarTerminal.Update(msg)
		a.ui.sidebarTerminal = newTerm
		// If the click returned a command (e.g., CreateNewTab from "+ New" button),
		// skip focusCmd to avoid double terminal creation.
		if cmd != nil {
			return cmd
		}
		return focusCmd
	case messages.PaneSidebar:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X, adjusted.Y = a.adjustSidebarMouseXY(adjusted.X, adjusted.Y)
		}
		newSidebar, cmd := a.ui.sidebar.Update(adjusted)
		a.ui.sidebar = newSidebar
		return common.SafeBatch(focusCmd, cmd)
	}
	return focusCmd
}

func (a *App) handleMouseMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return a.routeMouseClick(msg)
	case tea.MouseWheelMsg:
		return a.routeMouseWheel(msg)
	case tea.MouseMotionMsg:
		return a.routeMouseMotion(msg)
	case tea.MouseReleaseMsg:
		return a.routeMouseRelease(msg)
	default:
		return nil
	}
}

// routeMouseWheel routes mouse wheel events to the appropriate pane.
func (a *App) routeMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if a.prefixPaletteContainsPoint(msg.X, msg.Y) {
		// Palette wheel input is currently non-interactive; consume it so hidden
		// panes cannot scroll or steal focus while prefix mode is active.
		return nil
	}

	targetPane := a.focusedPane
	// Modal overlays and toast overlays do not consume wheel today, so preserve
	// focused-pane routing instead of hit-testing obscured panes beneath them.
	if !a.overlayVisible() && !a.toastCoversPoint(msg.X, msg.Y) {
		// Route wheel input by pointer target when possible so hovered panes
		// scroll without requiring a prior click. Fall back to keyboard focus
		// when the pointer is outside interactive pane geometry.
		hoverPane, hasTarget := a.paneForPoint(msg.X, msg.Y)
		if hasTarget {
			// Dashboard wheel handling activates rows, so do not retarget passive
			// hover wheel input into it from another pane.
			if hoverPane != messages.PaneDashboard || a.focusedPane == messages.PaneDashboard {
				if a.canRetargetWheelToPane(hoverPane) {
					targetPane = hoverPane
				}
			}
		}
	}

	var focusCmd tea.Cmd
	if targetPane != a.focusedPane {
		focusCmd = a.focusPaneOnWheel(targetPane)
	}

	switch targetPane {
	case messages.PaneDashboard:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X -= a.ui.layout.LeftGutter()
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newDashboard, cmd := a.ui.dashboard.Update(adjusted)
		a.ui.dashboard = newDashboard
		return common.SafeBatch(focusCmd, cmd)
	case messages.PaneCenter:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newCenter, cmd := a.ui.center.Update(adjusted)
		a.ui.center = newCenter
		return common.SafeBatch(focusCmd, cmd)
	case messages.PaneSidebarTerminal:
		newTerm, cmd := a.ui.sidebarTerminal.Update(msg)
		a.ui.sidebarTerminal = newTerm
		return common.SafeBatch(focusCmd, cmd)
	case messages.PaneSidebar:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X, adjusted.Y = a.adjustSidebarMouseXY(adjusted.X, adjusted.Y)
		}
		newSidebar, cmd := a.ui.sidebar.Update(adjusted)
		a.ui.sidebar = newSidebar
		return common.SafeBatch(focusCmd, cmd)
	}
	return nil
}

func (a *App) canRetargetWheelToPane(pane messages.PaneType) bool {
	switch pane {
	case messages.PaneCenter:
		return a.ui.center != nil && a.ui.center.CanConsumeWheel()
	case messages.PaneSidebar:
		return a.ui.sidebar != nil && a.ui.sidebar.CanConsumeWheel()
	case messages.PaneSidebarTerminal:
		return a.ui.sidebarTerminal != nil && a.ui.sidebarTerminal.CanConsumeWheel()
	default:
		return false
	}
}

// routeMouseMotion routes mouse motion events to the appropriate pane.
func (a *App) routeMouseMotion(msg tea.MouseMotionMsg) tea.Cmd {
	// Keep left-button drag motion bound to the pane focused on mouse-down.
	// Selection/edge-scroll logic depends on receiving out-of-bounds motion.
	targetPane := a.focusedPane
	if msg.Button != tea.MouseLeft {
		var ok bool
		targetPane, ok = a.paneForPoint(msg.X, msg.Y)
		if !ok {
			return nil
		}
	}
	switch targetPane {
	case messages.PaneDashboard:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X -= a.ui.layout.LeftGutter()
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newDashboard, cmd := a.ui.dashboard.Update(adjusted)
		a.ui.dashboard = newDashboard
		return cmd
	case messages.PaneCenter:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newCenter, cmd := a.ui.center.Update(adjusted)
		a.ui.center = newCenter
		return cmd
	case messages.PaneSidebarTerminal:
		newTerm, cmd := a.ui.sidebarTerminal.Update(msg)
		a.ui.sidebarTerminal = newTerm
		return cmd
	case messages.PaneSidebar:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X, adjusted.Y = a.adjustSidebarMouseXY(adjusted.X, adjusted.Y)
		}
		newSidebar, cmd := a.ui.sidebar.Update(adjusted)
		a.ui.sidebar = newSidebar
		return cmd
	}
	return nil
}

// routeMouseRelease routes mouse release events to the appropriate pane.
func (a *App) routeMouseRelease(msg tea.MouseReleaseMsg) tea.Cmd {
	// Keep left-button release bound to the pane focused on mouse-down so
	// cross-pane drags still finalize selection state in the source pane.
	targetPane := a.focusedPane
	if msg.Button != tea.MouseLeft {
		var ok bool
		targetPane, ok = a.paneForPoint(msg.X, msg.Y)
		if !ok {
			return nil
		}
	}
	switch targetPane {
	case messages.PaneDashboard:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X -= a.ui.layout.LeftGutter()
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newDashboard, cmd := a.ui.dashboard.Update(adjusted)
		a.ui.dashboard = newDashboard
		return cmd
	case messages.PaneCenter:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.Y -= a.ui.layout.TopGutter()
		}
		newCenter, cmd := a.ui.center.Update(adjusted)
		a.ui.center = newCenter
		return cmd
	case messages.PaneSidebarTerminal:
		newTerm, cmd := a.ui.sidebarTerminal.Update(msg)
		a.ui.sidebarTerminal = newTerm
		return cmd
	case messages.PaneSidebar:
		adjusted := msg
		if a.ui.layout != nil {
			adjusted.X, adjusted.Y = a.adjustSidebarMouseXY(adjusted.X, adjusted.Y)
		}
		newSidebar, cmd := a.ui.sidebar.Update(adjusted)
		a.ui.sidebar = newSidebar
		return cmd
	}
	return nil
}

func (a *App) paneForPoint(x, y int) (messages.PaneType, bool) {
	if a.ui == nil || a.ui.layout == nil {
		return paneNone, false
	}
	topGutter := a.ui.layout.TopGutter()
	height := a.ui.layout.Height()
	if y < topGutter || y >= topGutter+height {
		return paneNone, false
	}

	leftGutter := a.ui.layout.LeftGutter()
	if x < leftGutter {
		// Outer gutter is intentionally non-interactive; do not retarget focus.
		return paneNone, false
	}

	dashWidth := a.ui.layout.DashboardWidth()
	if x < leftGutter+dashWidth {
		return messages.PaneDashboard, true
	}

	// Keep hit-testing geometry in lockstep with app_view.go layout math:
	// dashboard, optional center (after gap), optional sidebar (after gap).
	centerStart := leftGutter + dashWidth
	if a.ui.layout.ShowCenter() {
		centerStart += a.ui.layout.GapX()
		centerEnd := centerStart + a.ui.layout.CenterWidth()
		if x >= centerStart && x < centerEnd {
			return messages.PaneCenter, true
		}
		centerStart = centerEnd
	}

	if !a.ui.layout.ShowSidebar() {
		return paneNone, false
	}
	sidebarStart := centerStart + a.ui.layout.GapX()
	sidebarEnd := sidebarStart + a.ui.layout.SidebarWidth()
	// Inter-pane gaps are intentionally non-interactive.
	if x < sidebarStart || x >= sidebarEnd {
		return paneNone, false
	}

	localY := y - topGutter
	topPaneHeight, _ := sidebarPaneHeights(height)
	if localY >= topPaneHeight {
		return messages.PaneSidebarTerminal, true
	}
	return messages.PaneSidebar, true
}

func (a *App) prefixPaletteContainsPoint(x, y int) bool {
	if !a.prefixActive || a.ui.width <= 0 || a.ui.height <= 0 {
		return false
	}
	palette := a.renderPrefixPalette()
	if palette == "" {
		return false
	}
	_, paletteHeight := viewDimensions(palette)
	if paletteHeight <= 0 {
		return false
	}
	paletteY := max(a.ui.height-paletteHeight, 0)
	return x >= 0 && x < a.ui.width && y >= paletteY && y < a.ui.height
}
