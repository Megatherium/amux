package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/app/orchestrator"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/common"
)

// criticalMsgDetector implements orchestrator.CriticalMessageDetector
// for the message pump. It is owned by the app package because it needs
// knowledge of both message types and UI-level critical message interfaces.
type criticalMsgDetector struct{}

// IsCritical reports whether msg should bypass the normal lossy queue
// and whether it is non-evicting (drop self vs evict others on full).
func (criticalMsgDetector) IsCritical(msg tea.Msg) (critical, nonEvicting bool) {
	if _, ok := msg.(common.CriticalExternalMsg); ok {
		_, nonEvicting := msg.(common.NonEvictingCriticalExternalMsg)
		return true, nonEvicting
	}
	switch msg.(type) {
	case messages.Error, messages.SidebarPTYStopped, center.PTYStopped:
		return true, false
	}
	return false, false
}

var _ orchestrator.CriticalMessageDetector = criticalMsgDetector{}

// SetMsgSender installs the send function for the external message pump.
func (a *App) SetMsgSender(send func(tea.Msg)) {
	if send == nil {
		return
	}
	a.oc().Pump.SetSender(send)
	if a.supervisor != nil {
		a.oc().Pump.Start(a.ctx, func(name string, fn func(context.Context) error) {
			a.supervisor.Start(name, fn)
		})
	} else {
		a.oc().Pump.Start(a.ctx, nil)
	}
}

// enqueueExternalMsg adds a message to the external message queue.
func (a *App) enqueueExternalMsg(msg tea.Msg) {
	a.oc().Pump.Enqueue(msg)
}
