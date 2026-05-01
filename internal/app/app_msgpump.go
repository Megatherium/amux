package app

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

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
