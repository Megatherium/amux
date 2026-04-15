package layout

import "testing"

func TestLayoutModes(t *testing.T) {
	m := NewManager()

	m.Resize(200, 40)
	if m.Mode() != LayoutThreePane {
		t.Fatalf("expected three-pane mode, got %v", m.Mode())
	}
	if !m.ShowSidebar() || !m.ShowCenter() {
		t.Fatalf("expected sidebar and center to be visible")
	}

	m.Resize(100, 40)
	if m.Mode() != LayoutTwoPane {
		t.Fatalf("expected two-pane mode, got %v", m.Mode())
	}
	if m.ShowSidebar() || !m.ShowCenter() {
		t.Fatalf("expected sidebar hidden and center visible")
	}

	m.Resize(50, 40)
	if m.Mode() != LayoutOnePane {
		t.Fatalf("expected one-pane mode, got %v", m.Mode())
	}
	if m.ShowCenter() {
		t.Fatalf("expected center hidden in one-pane mode")
	}
}

func TestLayoutWidthConstraints(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	if m.CenterWidth() < m.minChatWidth {
		t.Fatalf("center width should be >= minChatWidth")
	}
	if m.DashboardWidth() <= 0 {
		t.Fatalf("dashboard width should be > 0")
	}
}

func TestToggleBothHidesSidebars(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	if !m.ShowDashboard() {
		t.Fatal("dashboard should be visible before collapse")
	}
	if !m.ShowSidebar() {
		t.Fatal("sidebar should be visible before collapse")
	}

	m.ToggleBoth()

	if m.ShowDashboard() {
		t.Fatal("dashboard should be hidden after ToggleBoth")
	}
	if m.ShowSidebar() {
		t.Fatal("sidebar should be hidden after ToggleBoth")
	}
	if !m.ShowCenter() {
		t.Fatal("center should always be visible when collapsed")
	}
	if !m.IsCollapsed() {
		t.Fatal("IsCollapsed should return true after ToggleBoth")
	}
}

func TestToggleBothGivesCenterFullWidth(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	threePaneCenter := m.CenterWidth()
	m.ToggleBoth()

	if m.DashboardWidth() != 0 {
		t.Fatalf("dashboard width should be 0 when collapsed, got %d", m.DashboardWidth())
	}
	if m.SidebarWidth() != 0 {
		t.Fatalf("sidebar width should be 0 when collapsed, got %d", m.SidebarWidth())
	}
	if m.CenterWidth() <= threePaneCenter {
		t.Fatalf("collapsed center (%d) should be wider than three-pane center (%d)", m.CenterWidth(), threePaneCenter)
	}
	if m.CenterWidth() != m.totalWidth {
		t.Fatalf("collapsed center should equal totalWidth, got %d vs %d", m.CenterWidth(), m.totalWidth)
	}
}

func TestToggleBothRoundtrip(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	origDash := m.DashboardWidth()
	origCenter := m.CenterWidth()
	origSidebar := m.SidebarWidth()

	m.ToggleBoth()
	m.ToggleBoth()

	if m.DashboardWidth() != origDash {
		t.Fatalf("dashboard width not restored: got %d, want %d", m.DashboardWidth(), origDash)
	}
	if m.CenterWidth() != origCenter {
		t.Fatalf("center width not restored: got %d, want %d", m.CenterWidth(), origCenter)
	}
	if m.SidebarWidth() != origSidebar {
		t.Fatalf("sidebar width not restored: got %d, want %d", m.SidebarWidth(), origSidebar)
	}
	if m.IsCollapsed() {
		t.Fatal("IsCollapsed should be false after double toggle")
	}
}

func TestCollapseSurvivesResize(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)
	m.ToggleBoth()

	if !m.IsCollapsed() {
		t.Fatal("should be collapsed")
	}

	m.Resize(180, 40)

	if !m.IsCollapsed() {
		t.Fatal("collapse state should persist across resize")
	}
	if m.DashboardWidth() != 0 {
		t.Fatal("dashboard should remain hidden after resize while collapsed")
	}
}

func TestCollapseShowsCenterInOnePaneMode(t *testing.T) {
	m := NewManager()
	m.Resize(50, 40)

	if m.Mode() != LayoutOnePane {
		t.Fatal("expected one-pane mode")
	}
	if m.ShowCenter() {
		t.Fatal("center should be hidden in one-pane mode without collapse")
	}

	m.ToggleBoth()

	if !m.ShowCenter() {
		t.Fatal("center should be visible when collapsed even in one-pane mode")
	}
	if m.ShowDashboard() {
		t.Fatal("dashboard should be hidden when collapsed")
	}
}

func TestToggleDashboard(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	origSidebar := m.SidebarWidth()
	origCenter := m.CenterWidth()

	m.ToggleDashboard()

	if m.ShowDashboard() {
		t.Fatal("dashboard should be hidden after ToggleDashboard")
	}
	if !m.ShowSidebar() {
		t.Fatal("sidebar should remain visible after ToggleDashboard")
	}
	if m.SidebarWidth() != origSidebar {
		t.Fatalf("sidebar width should not change: got %d, want %d", m.SidebarWidth(), origSidebar)
	}
	if m.CenterWidth() <= origCenter {
		t.Fatalf("center should absorb dashboard width: got %d, had %d", m.CenterWidth(), origCenter)
	}
	if m.DashboardWidth() != 0 {
		t.Fatalf("dashboard width should be 0, got %d", m.DashboardWidth())
	}

	m.ToggleDashboard()

	if !m.ShowDashboard() {
		t.Fatal("dashboard should be visible after second ToggleDashboard")
	}
}

func TestToggleSidebar(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	origDash := m.DashboardWidth()
	origCenter := m.CenterWidth()

	m.ToggleSidebar()

	if m.ShowSidebar() {
		t.Fatal("sidebar should be hidden after ToggleSidebar")
	}
	if !m.ShowDashboard() {
		t.Fatal("dashboard should remain visible after ToggleSidebar")
	}
	if m.DashboardWidth() != origDash {
		t.Fatalf("dashboard width should not change: got %d, want %d", m.DashboardWidth(), origDash)
	}
	if m.CenterWidth() <= origCenter {
		t.Fatalf("center should absorb sidebar width: got %d, had %d", m.CenterWidth(), origCenter)
	}
	if m.SidebarWidth() != 0 {
		t.Fatalf("sidebar width should be 0, got %d", m.SidebarWidth())
	}

	m.ToggleSidebar()

	if !m.ShowSidebar() {
		t.Fatal("sidebar should be visible after second ToggleSidebar")
	}
}

func TestIndependentCollapse(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	m.ToggleDashboard()

	if m.ShowDashboard() {
		t.Fatal("dashboard should be hidden")
	}
	if !m.ShowSidebar() {
		t.Fatal("sidebar should still be visible")
	}

	m.ToggleSidebar()

	if m.ShowSidebar() {
		t.Fatal("sidebar should now be hidden too")
	}
	if !m.IsCollapsed() {
		t.Fatal("both collapsed means IsCollapsed should be true")
	}
}

func TestToggleBothUnifiesAfterIndividualCollapse(t *testing.T) {
	m := NewManager()
	m.Resize(200, 40)

	// Individually collapse dashboard only
	m.ToggleDashboard()
	if m.ShowDashboard() {
		t.Fatal("dashboard should be hidden")
	}
	if !m.ShowSidebar() {
		t.Fatal("sidebar should still be visible")
	}

	// ToggleBoth should collapse both (unify), not flip independently
	m.ToggleBoth()
	if m.ShowDashboard() {
		t.Fatal("dashboard should remain hidden after ToggleBoth")
	}
	if m.ShowSidebar() {
		t.Fatal("sidebar should now be hidden after ToggleBoth (unify)")
	}

	// ToggleBoth again should restore both
	m.ToggleBoth()
	if !m.ShowDashboard() {
		t.Fatal("dashboard should be visible after second ToggleBoth")
	}
	if !m.ShowSidebar() {
		t.Fatal("sidebar should be visible after second ToggleBoth")
	}
}

func TestCollapseOnePaneNoPhantomGap(t *testing.T) {
	m := NewManager()
	m.Resize(50, 40) // one-pane mode

	if m.Mode() != LayoutOnePane {
		t.Fatal("expected one-pane mode")
	}

	m.ToggleBoth()

	// In one-pane mode there are no gaps, so center should equal totalWidth exactly
	if m.CenterWidth() != m.totalWidth {
		t.Fatalf("one-pane collapse should give center totalWidth, got %d vs %d", m.CenterWidth(), m.totalWidth)
	}
}
