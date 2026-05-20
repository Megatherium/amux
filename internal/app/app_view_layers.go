package app

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/compositor"
)

// renderDashboardLayer draws the leftmost dashboard pane if layout allows.
func (a *App) renderDashboardLayer(canvas *lipgloss.Canvas, leftGutter, topGutter, dashWidth int) {
	if !a.ui.layout.ShowDashboard() {
		return
	}
	dashHeight := a.ui.layout.Height()
	dashContentWidth := clampMin(dashWidth-3, 1)
	dashContentHeight := clampMin(dashHeight-2, 1)
	dashContent := clampLines(a.ui.dashboard.View(), dashContentWidth, dashContentHeight)
	if dashDrawable := a.ui.dashboardContent.get(dashContent, leftGutter+1, topGutter+1); dashDrawable != nil {
		canvas.Compose(dashDrawable)
	}
	for _, border := range a.ui.dashboardBorders.get(leftGutter, topGutter, dashWidth, dashHeight) {
		canvas.Compose(border)
	}
}

// renderCenterPaneLayer draws the center pane if layout allows.
func (a *App) renderCenterPaneLayer(
	canvas *lipgloss.Canvas,
	leftGutter, topGutter, dashWidth int,
	blockingOverlayVisible bool,
	setTerminalCursor func(x, y int),
) {
	if !a.ui.layout.ShowCenter() {
		return
	}
	centerGap := 0
	if a.ui.layout.ShowDashboard() {
		centerGap = a.ui.layout.GapX()
	}
	centerX := leftGutter + dashWidth + centerGap
	centerWidth := a.ui.layout.CenterWidth()
	centerHeight := a.ui.layout.Height()

	// Check if we can use VTermLayer for direct cell rendering
	centerOwnsCursor := a.oc().Focus.FocusedPane == messages.PaneCenter && !blockingOverlayVisible
	if termLayer := a.ui.center.TerminalLayerWithCursorOwner(centerOwnsCursor); termLayer != nil && a.ui.center.HasTabs() && !a.ui.center.HasDiffViewer() {
		// Get terminal viewport from center model (accounts for borders, tab bar, help lines)
		termOffsetX, termOffsetY, termW, termH := a.ui.center.TerminalViewport()
		termX := centerX + termOffsetX
		termY := topGutter + termOffsetY
		if centerOwnsCursor && termLayer.Snap != nil {
			snap := termLayer.Snap
			if snap.ShowCursor && !snap.CursorHidden && snap.ViewOffset == 0 &&
				snap.CursorX >= 0 && snap.CursorY >= 0 &&
				snap.CursorX < termW && snap.CursorY < termH {
				setTerminalCursor(termX+snap.CursorX, termY+snap.CursorY)
				// Keep exactly one visible cursor by delegating to the hardware cursor.
				// This shallow copy is intentional: only ShowCursor changes here, and
				// the snapshot screen data remains read-only for rendering.
				snapCopy := *snap
				snapCopy.ShowCursor = false
				termLayer = compositor.NewVTermLayer(&snapCopy)
			}
		}

		// Compose terminal layer first; chrome is drawn on top without clearing the content area.
		positionedTermLayer := &compositor.PositionedVTermLayer{
			VTermLayer: termLayer,
			PosX:       termX,
			PosY:       termY,
			Width:      termW,
			Height:     termH,
		}
		canvas.Compose(positionedTermLayer)

		// Draw borders without touching the content area.
		for _, border := range a.ui.centerBorders.get(centerX, topGutter, centerWidth, centerHeight) {
			canvas.Compose(border)
		}

		contentWidth := clampMin(a.ui.center.ContentWidth(), 1)
		if contentWidth < 1 {
			contentWidth = 1
		}

		// Tab bar (top of content area).
		tabBar := clampLines(a.ui.center.TabBarView(), contentWidth, termOffsetY-1)
		if tabBarDrawable := a.ui.centerTabBar.get(tabBar, termX, topGutter+1); tabBarDrawable != nil {
			canvas.Compose(tabBarDrawable)
		}

		// Status line (directly below terminal content).
		if status := clampLines(a.ui.center.ActiveTerminalStatusLine(), contentWidth, 1); status != "" {
			if statusDrawable := a.ui.centerStatus.get(status, termX, termY+termH); statusDrawable != nil {
				canvas.Compose(statusDrawable)
			}
		}

		// Help lines at bottom of pane.
		if helpLines := a.ui.center.HelpLines(contentWidth); len(helpLines) > 0 {
			helpContent := clampLines(strings.Join(helpLines, "\n"), contentWidth, len(helpLines))
			helpY := topGutter + centerHeight - 1 - len(helpLines)
			if helpY > termY {
				if helpDrawable := a.ui.centerHelp.get(helpContent, termX, helpY); helpDrawable != nil {
					canvas.Compose(helpDrawable)
				}
			}
		}
	} else {
		// Fallback to string-based rendering with borders (no caching - content changes)
		a.ui.centerChrome.Invalidate()
		var centerContent string
		if a.ui.center.HasTabs() {
			centerContent = a.ui.center.View()
		} else {
			centerContent = a.renderCenterPaneContent()
		}
		centerView := buildBorderedPane(centerContent, centerWidth, centerHeight)
		centerDrawable := compositor.NewStringDrawable(clampPane(centerView, centerWidth, centerHeight), centerX, topGutter)
		canvas.Compose(centerDrawable)
	}
}

// renderSidebarLayer draws the rightmost sidebar pane if layout allows.
func (a *App) renderSidebarLayer(
	canvas *lipgloss.Canvas,
	leftGutter, topGutter int,
	blockingOverlayVisible bool,
	setTerminalCursor func(x, y int),
) {
	if !a.ui.layout.ShowSidebar() {
		return
	}
	sidebarX := leftGutter + a.ui.layout.DashboardWidth()
	if a.ui.layout.ShowDashboard() && a.ui.layout.ShowCenter() {
		sidebarX += a.ui.layout.GapX()
	}
	if a.ui.layout.ShowCenter() {
		sidebarX += a.ui.layout.CenterWidth()
	}
	sidebarX += a.ui.layout.GapX()
	sidebarWidth := a.ui.layout.SidebarWidth()
	sidebarHeight := a.ui.layout.Height()
	topPaneHeight, bottomPaneHeight := sidebarPaneHeights(sidebarHeight)
	if bottomPaneHeight > 0 {
		contentWidth := clampMin(sidebarWidth-4, 1)
		if contentWidth < 1 {
			contentWidth = 1
		}

		if topPaneHeight > 0 {
			topContentHeight := clampMin(topPaneHeight-2, 1)
			if topContentHeight < 1 {
				topContentHeight = 1
			}

			// Sidebar tab bar (Changes/Project tabs)
			tabBar := a.ui.sidebar.TabBarView()
			tabBarHeight := 0
			if tabBar != "" {
				tabBarHeight = 1
				tabBarContent := clampLines(tabBar, contentWidth, 1)
				tabBarY := topGutter + 1 // Inside the border
				if tabBarDrawable := a.ui.sidebarTopTabBar.get(tabBarContent, sidebarX+2, tabBarY); tabBarDrawable != nil {
					canvas.Compose(tabBarDrawable)
				}
			}

			// Sidebar content (below tab bar)
			sidebarContentHeight := clampMin(topContentHeight-tabBarHeight, 1)
			topContent := clampLines(a.ui.sidebar.ContentView(), contentWidth, sidebarContentHeight)
			if topDrawable := a.ui.sidebarTopContent.get(topContent, sidebarX+2, topGutter+1+tabBarHeight); topDrawable != nil {
				canvas.Compose(topDrawable)
			}
			for _, border := range a.ui.sidebarTopBorders.get(sidebarX, topGutter, sidebarWidth, topPaneHeight) {
				canvas.Compose(border)
			}
		}

		bottomY := topGutter + topPaneHeight
		bottomContentHeight := clampMin(bottomPaneHeight-2, 1)
		if bottomContentHeight < 1 {
			bottomContentHeight = 1
		}

		sidebarOwnsCursor := a.oc().Focus.FocusedPane == messages.PaneSidebarTerminal && !blockingOverlayVisible
		if termLayer := a.ui.sidebarTerminal.TerminalLayerWithCursorOwner(sidebarOwnsCursor); termLayer != nil {
			originX, originY := a.ui.sidebarTerminal.TerminalOrigin()
			termW, termH := a.ui.sidebarTerminal.TerminalSize()
			if termW > contentWidth {
				termW = contentWidth
			}
			if termH > bottomContentHeight {
				termH = bottomContentHeight
			}

			// Tab bar (above terminal content) - compact single line
			tabBar := a.ui.sidebarTerminal.TabBarView()
			tabBarHeight := 0
			if tabBar != "" {
				tabBarHeight = 1
				tabBarContent := clampLines(tabBar, contentWidth, 1)
				tabBarY := bottomY + 1 // Inside the border
				if tabBarDrawable := a.ui.sidebarBottomTabBar.get(tabBarContent, originX, tabBarY); tabBarDrawable != nil {
					canvas.Compose(tabBarDrawable)
				}
			}

			status := clampLines(a.ui.sidebarTerminal.StatusLine(), contentWidth, 1)
			helpLines := a.ui.sidebarTerminal.HelpLines(contentWidth)
			statusLines := 0
			if status != "" {
				statusLines = 1
			}
			maxHelpHeight := bottomContentHeight - statusLines - tabBarHeight
			if maxHelpHeight < 0 {
				maxHelpHeight = 0
			}
			if len(helpLines) > maxHelpHeight {
				helpLines = helpLines[:maxHelpHeight]
			}
			maxTermHeight := bottomContentHeight - statusLines - len(helpLines) - tabBarHeight
			if maxTermHeight < 0 {
				maxTermHeight = 0
			}
			if termH > maxTermHeight {
				termH = maxTermHeight
			}
			if sidebarOwnsCursor && termLayer.Snap != nil {
				snap := termLayer.Snap
				if snap.ShowCursor && !snap.CursorHidden && snap.ViewOffset == 0 &&
					snap.CursorX >= 0 && snap.CursorY >= 0 &&
					snap.CursorX < termW && snap.CursorY < termH {
					setTerminalCursor(originX+snap.CursorX, originY+snap.CursorY)
					// Keep exactly one visible cursor by delegating to the hardware cursor.
					// This shallow copy is intentional: only ShowCursor changes here, and
					// the snapshot screen data remains read-only for rendering.
					snapCopy := *snap
					snapCopy.ShowCursor = false
					termLayer = compositor.NewVTermLayer(&snapCopy)
				}
			}

			positioned := &compositor.PositionedVTermLayer{
				VTermLayer: termLayer,
				PosX:       originX,
				PosY:       originY,
				Width:      termW,
				Height:     termH,
			}
			canvas.Compose(positioned)

			if status != "" {
				if statusDrawable := a.ui.sidebarBottomStatus.get(status, originX, originY+termH); statusDrawable != nil {
					canvas.Compose(statusDrawable)
				}
			}

			if len(helpLines) > 0 {
				helpContent := clampLines(strings.Join(helpLines, "\n"), contentWidth, len(helpLines))
				helpY := originY + bottomContentHeight - len(helpLines) - tabBarHeight
				if helpDrawable := a.ui.sidebarBottomHelp.get(helpContent, originX, helpY); helpDrawable != nil {
					canvas.Compose(helpDrawable)
				}
			} else if status == "" && bottomContentHeight > termH+tabBarHeight {
				blank := strings.Repeat(" ", contentWidth)
				if blankDrawable := a.ui.sidebarBottomHelp.get(blank, originX, originY+bottomContentHeight-1-tabBarHeight); blankDrawable != nil {
					canvas.Compose(blankDrawable)
				}
			}
		} else {
			bottomContent := clampLines(a.ui.sidebarTerminal.View(), contentWidth, bottomContentHeight)
			if bottomDrawable := a.ui.sidebarBottomContent.get(bottomContent, sidebarX+2, bottomY+1); bottomDrawable != nil {
				canvas.Compose(bottomDrawable)
			}
		}
		for _, border := range a.ui.sidebarBottomBorders.get(sidebarX, bottomY, sidebarWidth, bottomPaneHeight) {
			canvas.Compose(border)
		}
	}
}

// clampMin returns v if it's greater than or equal to limit, otherwise limit.
func clampMin(v, limit int) int {
	if v < limit {
		return limit
	}
	return v
}
