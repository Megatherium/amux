package dashboard

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/data"
	"github.com/andyrewlee/amux/internal/messages"
)

// setupClickTestModelWithTickets creates a model with ticket rows for click testing.
func setupClickTestModelWithTickets() *Model {
	p, ts := makeProjectWithTickets()
	m := New()
	m.SetSize(60, 20)
	m.showKeymapHints = false
	m.SetProjects([]data.Project{p})
	m.SetTickets(p.Path, ts)
	_ = m.View()
	return m
}

// findFirstTicketRow returns the row index of the first RowTicket, or -1 if none.
func findFirstTicketRow(m *Model) int {
	for i, row := range m.rows {
		if row.Type == RowTicket {
			return i
		}
	}
	return -1
}

func TestSingleClickOnTicketShowsPreview(t *testing.T) {
	m := setupClickTestModelWithTickets()
	ticketIdx := findFirstTicketRow(m)
	if ticketIdx < 0 {
		t.Fatal("no ticket rows in model")
	}

	screenY := ticketIdx - m.scrollOffset + 1
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenY}

	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected command from single click on ticket row")
	}
	msg := cmd()
	preview, ok := msg.(messages.TicketPreviewMsg)
	if !ok {
		t.Fatalf("expected TicketPreviewMsg, got %T", msg)
	}
	if preview.Ticket == nil {
		t.Fatal("expected non-nil ticket in preview")
	}
}

func TestDoubleClickOnTicketStartsDraft(t *testing.T) {
	m := setupClickTestModelWithTickets()
	ticketIdx := findFirstTicketRow(m)
	if ticketIdx < 0 {
		t.Fatal("no ticket rows in model")
	}

	screenY := ticketIdx - m.scrollOffset + 1
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenY}

	// First click sets up double-click state
	_, _ = m.Update(click)

	// Second click (same row, within window) -> TicketSelectedMsg
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected command from double-click on ticket row")
	}
	msg := cmd()
	sel, ok := msg.(messages.TicketSelectedMsg)
	if !ok {
		t.Fatalf("expected TicketSelectedMsg on double-click, got %T", msg)
	}
	if sel.Ticket == nil {
		t.Fatal("expected non-nil ticket in selected msg")
	}
}

func TestDoubleClickStateResetsAfterDraft(t *testing.T) {
	m := setupClickTestModelWithTickets()
	ticketIdx := findFirstTicketRow(m)
	if ticketIdx < 0 {
		t.Fatal("no ticket rows in model")
	}

	screenY := ticketIdx - m.scrollOffset + 1
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenY}

	// First click
	_, _ = m.Update(click)
	// Second click (double-click triggers draft)
	_, _ = m.Update(click)

	// State should be reset after double-click
	if m.lastClickRow != -1 {
		t.Errorf("lastClickRow should be -1 after double-click, got %d", m.lastClickRow)
	}
	if !m.lastClickTime.IsZero() {
		t.Error("lastClickTime should be zero after double-click")
	}

	// Next single click should be a preview again (not another double-click)
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected command from subsequent click after state reset")
	}
	msg := cmd()
	if _, ok := msg.(messages.TicketPreviewMsg); !ok {
		t.Fatalf("expected TicketPreviewMsg after state reset, got %T", msg)
	}
}

func TestClickDifferentTicketRowsNotDoubleClick(t *testing.T) {
	m := setupClickTestModelWithTickets()

	// Find two different ticket rows
	var ticketIdx1, ticketIdx2 int
	found := 0
	for i, row := range m.rows {
		if row.Type == RowTicket {
			if found == 0 {
				ticketIdx1 = i
				found++
			} else {
				ticketIdx2 = i
				break
			}
		}
	}
	if ticketIdx2 == 0 {
		t.Fatal("need at least 2 ticket rows for this test")
	}

	screenY1 := ticketIdx1 - m.scrollOffset + 1
	click1 := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenY1}

	screenY2 := ticketIdx2 - m.scrollOffset + 1
	click2 := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenY2}

	// Click first ticket row -> preview
	_, cmd1 := m.Update(click1)
	if cmd1 == nil {
		t.Fatal("expected command from first click")
	}
	msg1 := cmd1()
	if _, ok := msg1.(messages.TicketPreviewMsg); !ok {
		t.Fatalf("expected TicketPreviewMsg for first click, got %T", msg1)
	}

	// Click different ticket row -> still preview (not double-click)
	_, cmd2 := m.Update(click2)
	if cmd2 == nil {
		t.Fatal("expected command from second click on different row")
	}
	msg2 := cmd2()
	if _, ok := msg2.(messages.TicketPreviewMsg); !ok {
		t.Fatalf("expected TicketPreviewMsg for different row click, got %T", msg2)
	}
}

func TestClickAfterWindowExpiresIsNotDoubleClick(t *testing.T) {
	m := setupClickTestModelWithTickets()
	ticketIdx := findFirstTicketRow(m)
	if ticketIdx < 0 {
		t.Fatal("no ticket rows in model")
	}

	screenY := ticketIdx - m.scrollOffset + 1
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenY}

	// First click sets lastClickTime
	_, _ = m.Update(click)

	// Advance lastClickTime beyond the double-click window
	m.lastClickTime = m.lastClickTime.Add(-doubleClickWindow - time.Second)

	// Second click should be treated as single click (preview)
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected command from click after window expired")
	}
	msg := cmd()
	if _, ok := msg.(messages.TicketPreviewMsg); !ok {
		t.Fatalf("expected TicketPreviewMsg after window expired, got %T", msg)
	}
}

func TestNonTicketRowClickUsesHandleEnter(t *testing.T) {
	m := setupClickTestModelWithTickets()

	// Find a workspace row (non-ticket selectable row)
	var wsIdx int
	for i, row := range m.rows {
		if row.Type == RowWorkspace {
			wsIdx = i
			break
		}
	}
	if wsIdx == 0 {
		t.Fatal("no workspace rows in model with tickets")
	}

	screenY := wsIdx - m.scrollOffset + 1
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenY}

	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected command from click on workspace row")
	}
	msg := cmd()
	// Non-ticket rows should emit WorkspaceActivated, not TicketPreviewMsg
	act, ok := msg.(messages.WorkspaceActivated)
	if !ok {
		t.Fatalf("expected WorkspaceActivated for workspace row, got %T", msg)
	}
	if act.Workspace == nil {
		t.Fatal("expected non-nil workspace in activation msg")
	}
}

func TestNonTicketClickResetsDoubleClickState(t *testing.T) {
	m := setupClickTestModelWithTickets()

	// First click a ticket row to set double-click state
	ticketIdx := findFirstTicketRow(m)
	if ticketIdx < 0 {
		t.Fatal("no ticket rows in model")
	}
	screenYT := ticketIdx - m.scrollOffset + 1
	clickTicket := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenYT}
	_, _ = m.Update(clickTicket)

	// Verify state is set
	if m.lastClickRow == -1 {
		t.Fatal("lastClickRow should be set after clicking ticket row")
	}

	// Now click a non-ticket row (Home row)
	screenYHome := 0 - m.scrollOffset + 1
	clickHome := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: screenYHome}
	_, _ = m.Update(clickHome)

	// State should be cleared
	if m.lastClickRow != -1 {
		t.Errorf("lastClickRow should be -1 after non-ticket click, got %d", m.lastClickRow)
	}
	if !m.lastClickTime.IsZero() {
		t.Error("lastClickTime should be zero after non-ticket click")
	}
}

func TestRebuildRowsResetsDoubleClickState(t *testing.T) {
	m := setupClickTestModelWithTickets()

	// Set double-click state
	m.lastClickRow = 3
	m.lastClickTime = time.Now()

	// Trigger rebuild via SetTickets
	p, ts := makeProjectWithTickets()
	m.SetTickets(p.Path, ts)

	// State should be cleared after rebuild
	if m.lastClickRow != -1 {
		t.Errorf("lastClickRow should be -1 after rebuildRows, got %d", m.lastClickRow)
	}
	if !m.lastClickTime.IsZero() {
		t.Error("lastClickTime should be zero after rebuildRows")
	}
}
