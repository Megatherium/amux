package app

import (
	"strings"
	"testing"
	"time"

	"github.com/andyrewlee/amux/internal/ui/common"
)

func TestViewHidesTerminalCursorWhenSettingsOverlayIsVisible(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Tabs:   1,
		Width:  160,
		Height: 48,
	})
	if err != nil {
		t.Fatalf("expected harness creation to succeed: %v", err)
	}
	if len(h.tabs) != 1 || h.tabs[0] == nil || h.tabs[0].Terminal == nil {
		t.Fatal("expected center harness terminal")
	}
	h.tabs[0].Terminal.CursorX = 1
	h.tabs[0].Terminal.CursorY = h.tabs[0].Terminal.Height - 1

	base := h.Render()
	if base.Cursor == nil {
		t.Fatal("expected visible terminal cursor before overlay")
	}

	h.app.ui.settingsDialog = common.NewSettingsDialog(common.ThemeTokyoNight)
	h.app.ui.settingsDialog.Show()
	h.app.ui.settingsDialog.SetSize(h.app.ui.width, h.app.ui.height)

	overlay := h.Render()
	if overlay.Cursor != nil {
		t.Fatal("expected terminal cursor to be hidden while settings overlay is visible")
	}
}

func TestViewKeepsTerminalCursorWhenOnlyToastIsVisible(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Tabs:   1,
		Width:  160,
		Height: 48,
	})
	if err != nil {
		t.Fatalf("expected harness creation to succeed: %v", err)
	}
	if len(h.tabs) != 1 || h.tabs[0] == nil || h.tabs[0].Terminal == nil {
		t.Fatal("expected center harness terminal")
	}
	h.tabs[0].Terminal.CursorX = 1
	h.tabs[0].Terminal.CursorY = h.tabs[0].Terminal.Height - 1

	base := h.Render()
	if base.Cursor == nil {
		t.Fatal("expected visible terminal cursor before toast")
	}

	_ = h.app.ui.toast.ShowInfo("copy complete")

	toastView := h.Render()
	if toastView.Cursor == nil {
		t.Fatal("expected terminal cursor to remain visible while toast is shown")
	}
}

func TestViewHidesTerminalCursorWhenToastCoversIt(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Tabs:   1,
		Width:  95,
		Height: 8,
	})
	if err != nil {
		t.Fatalf("expected harness creation to succeed: %v", err)
	}
	if len(h.tabs) != 1 || h.tabs[0] == nil || h.tabs[0].Terminal == nil {
		t.Fatal("expected center harness terminal")
	}

	_ = h.app.ui.toast.Show("copy complete", common.ToastInfo, time.Minute)

	termOffsetX, termOffsetY, termW, termH := h.app.ui.center.TerminalViewport()
	centerX := h.app.ui.layout.LeftGutter() + h.app.ui.layout.DashboardWidth() + h.app.ui.layout.GapX()
	termX := centerX + termOffsetX
	termY := h.app.ui.layout.TopGutter() + termOffsetY

	toastView := h.app.ui.toast.View()
	if toastView == "" {
		t.Fatal("expected visible toast")
	}
	toastW, toastH := viewDimensions(toastView)
	toastX := (h.app.ui.width - toastW) / 2
	toastY := h.app.ui.height - 2

	overlapLeft := termX
	if toastX > overlapLeft {
		overlapLeft = toastX
	}
	overlapTop := termY
	if toastY > overlapTop {
		overlapTop = toastY
	}
	overlapRight := termX + termW
	if toastX+toastW < overlapRight {
		overlapRight = toastX + toastW
	}
	overlapBottom := termY + termH
	if toastY+toastH < overlapBottom {
		overlapBottom = toastY + toastH
	}
	if overlapLeft >= overlapRight || overlapTop >= overlapBottom {
		t.Fatal("expected toast and terminal viewport to overlap in test setup")
	}

	h.tabs[0].Terminal.CursorX = overlapLeft - termX
	h.tabs[0].Terminal.CursorY = overlapTop - termY

	view := h.Render()
	if view.Cursor != nil {
		t.Fatal("expected hardware cursor to stay hidden when a toast covers the cursor cell")
	}
}

func TestViewHardwareCursorDelegationDoesNotMutateCachedSnapshot(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Tabs:   1,
		Width:  160,
		Height: 48,
	})
	if err != nil {
		t.Fatalf("expected harness creation to succeed: %v", err)
	}
	if len(h.tabs) != 1 || h.tabs[0] == nil || h.tabs[0].Terminal == nil {
		t.Fatal("expected center harness terminal")
	}
	h.tabs[0].Terminal.CursorX = 1
	h.tabs[0].Terminal.CursorY = h.tabs[0].Terminal.Height - 1

	view := h.Render()
	if view.Cursor == nil {
		t.Fatal("expected hardware cursor delegation during render")
	}

	layer := h.app.ui.center.TerminalLayerWithCursorOwner(true)
	if layer == nil || layer.Snap == nil {
		t.Fatal("expected cached terminal layer snapshot after render")
	}
	if !layer.Snap.ShowCursor {
		t.Fatal("expected cached snapshot to retain software cursor visibility after hardware delegation")
	}
}

func TestViewHidesOverlayCursorWhenToastCoversIt(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Tabs:   1,
		Width:  80,
		Height: 24,
	})
	if err != nil {
		t.Fatalf("expected harness creation to succeed: %v", err)
	}

	dialog := common.NewInputDialog("rename", "Rename", "file name")
	dialog.Show()
	h.app.ui.dialog = dialog

	_ = h.app.ui.toast.ShowInfo(strings.Repeat("toast ", 12))

	covered := false
	for height := 4; height <= 24; height++ {
		h.app.ui.height = height
		h.app.ui.layout.Resize(h.app.ui.width, h.app.ui.height)
		h.app.updateLayout()
		dialog.SetSize(h.app.ui.width, h.app.ui.height)

		cursor := h.app.overlayCursor()
		if cursor != nil && h.app.toastCoversPoint(cursor.X, cursor.Y) {
			covered = true
			break
		}
	}
	if !covered {
		t.Fatal("expected toast to cover the overlay cursor in test setup")
	}

	view := h.Render()
	if view.Cursor != nil {
		t.Fatal("expected overlay cursor to stay hidden when a toast covers the cursor cell")
	}
}

func TestViewWrapsRenderedFrameInSynchronizedOutputMarkers(t *testing.T) {
	h, err := NewHarness(HarnessOptions{
		Mode:   HarnessCenter,
		Tabs:   1,
		Width:  160,
		Height: 48,
	})
	if err != nil {
		t.Fatalf("expected harness creation to succeed: %v", err)
	}

	view := h.Render()
	if !strings.HasPrefix(view.Content, syncBegin) {
		t.Fatal("expected rendered frame to start with DEC 2026 sync begin marker")
	}
	if !strings.HasSuffix(view.Content, syncEnd) {
		t.Fatal("expected rendered frame to end with DEC 2026 sync end marker")
	}
}
