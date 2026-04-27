package app

import (
	"github.com/andyrewlee/amux/internal/config"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/common"
	"github.com/andyrewlee/amux/internal/ui/dashboard"
	"github.com/andyrewlee/amux/internal/ui/layout"
	"github.com/andyrewlee/amux/internal/ui/sidebar"
)

// UICompositor holds the core UI component models that together form the
// main screen layout. It is extracted from the App god object as part of
// a gradual decomposition; fields that are lazy-created (dialog, filePicker,
// settingsDialog) start as nil and are instantiated on first use.
type UICompositor struct {
	layout          *layout.Manager
	dashboard       *dashboard.Model
	center          *center.Model
	sidebar         *sidebar.TabbedSidebar
	sidebarTerminal *sidebar.TerminalModel
	dialog          *common.Dialog         // lazy-created
	filePicker      *common.FilePicker     // lazy-created
	settingsDialog  *common.SettingsDialog // lazy-created
	toast           *common.ToastModel
}

// newUICompositor creates a UICompositor with all non-lazy fields
// initialized. Lazy fields (dialog, filePicker, settingsDialog) are
// left nil and will be created on first use.
func newUICompositor(cfg *config.Config) *UICompositor {
	return &UICompositor{
		layout:          layout.NewManager(),
		dashboard:       dashboard.New(),
		center:          center.New(cfg),
		sidebar:         sidebar.NewTabbedSidebar(),
		sidebarTerminal: sidebar.NewTerminalModel(),
		toast:           common.NewToastModel(),
	}
}
