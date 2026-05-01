package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/andyrewlee/amux/internal/logging"
	"github.com/andyrewlee/amux/internal/messages"
	"github.com/andyrewlee/amux/internal/perf"
	"github.com/andyrewlee/amux/internal/safego"
	"github.com/andyrewlee/amux/internal/ui/center"
	"github.com/andyrewlee/amux/internal/ui/common"
)

const (
	// ExternalMsgBuffer is the size of the external message channel.
	ExternalMsgBuffer = 4096
	// ExternalCriticalBuffer is the size of the external critical message channel.
	ExternalCriticalBuffer = 512
)

// MessagePump manages the external message queue used by PTY readers
// and background workers to safely enqueue messages into the Bubble Tea
// update loop. Extracted from app_msgpump.go.
type MessagePump struct {
	msgs     chan tea.Msg
	critical chan tea.Msg
	sender   func(tea.Msg)
	once     sync.Once
}

// NewMessagePump creates an initialized MessagePump with default buffer sizes.
func NewMessagePump() *MessagePump {
	return NewMessagePumpWithSize(ExternalMsgBuffer, ExternalCriticalBuffer)
}

// NewMessagePumpWithSize creates a MessagePump with custom buffer sizes (for testing).
func NewMessagePumpWithSize(normalSize, criticalSize int) *MessagePump {
	return &MessagePump{
		msgs:     make(chan tea.Msg, normalSize),
		critical: make(chan tea.Msg, criticalSize),
	}
}

// Channels returns the two external message channels.
// The caller must set up reading from these channels.
func (mp *MessagePump) Channels() (normal, critical chan tea.Msg) {
	return mp.msgs, mp.critical
}

// SetSender installs the send function that delivers messages into
// the Bubble Tea update loop. Must be called exactly once.
func (mp *MessagePump) SetSender(send func(tea.Msg)) {
	if send == nil {
		return
	}
	mp.once.Do(func() {
		mp.sender = send
	})
}

// Enqueue adds a message to the external queue. Critical messages
// (Error, PTY stopped) are placed on the high-priority channel.
func (mp *MessagePump) Enqueue(msg tea.Msg) {
	_ = mp.tryEnqueue(msg)
}

// TryEnqueue attempts to enqueue a message and returns true on success.
func (mp *MessagePump) TryEnqueue(msg tea.Msg) bool {
	return mp.tryEnqueue(msg)
}

func (mp *MessagePump) tryEnqueue(msg tea.Msg) bool {
	if msg == nil {
		return false
	}
	if isCriticalExternalMsg(msg) {
		_, nonEvicting := msg.(common.NonEvictingCriticalExternalMsg)
		select {
		case mp.critical <- msg:
			return true
		default:
			if nonEvicting {
				perf.Count("external_msg_drop_critical", 1)
				return false
			}
			// Critical channel full - try to drop a non-critical message to make room.
			select {
			case <-mp.msgs:
				perf.Count("external_msg_drop_noncritical", 1)
			default:
			}
			select {
			case mp.critical <- msg:
				return true
			default:
				perf.Count("external_msg_drop_critical", 1)
				return false
			}
		}
	}
	select {
	case mp.msgs <- msg:
		return true
	default:
		perf.Count("external_msg_drop", 1)
		return false
	}
}

// Run pumps messages from the external channels to the installed sender.
// Blocks until ctx is canceled or a channel is closed.
func (mp *MessagePump) Run(ctx context.Context) error {
	for {
		// Fast-path: drain critical messages first (non-blocking).
		select {
		case msg, ok := <-mp.critical:
			if !ok {
				return nil
			}
			if msg != nil && mp.sender != nil {
				mp.sender(msg)
			} else if msg != nil {
				logging.Warn("critical message dropped: sender not initialized")
			}
			continue
		default:
		}
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-mp.critical:
			if !ok {
				return nil
			}
			if msg == nil {
				continue
			}
			if mp.sender == nil {
				logging.Warn("critical message dropped: sender not initialized")
				continue
			}
			mp.sender(msg)
		case msg, ok := <-mp.msgs:
			if !ok {
				return nil
			}
			if msg == nil {
				continue
			}
			if mp.sender == nil {
				logging.Warn("message dropped: sender not initialized")
				continue
			}
			mp.sender(msg)
		}
	}
}

// StartFunc is a function that starts a named background worker.
type StartFunc func(name string, fn func(context.Context) error)

// SetErrorHandlerFunc sets an error handler callback.
type SetErrorHandlerFunc func(func(name string, err error))

// Start begins the message pump in a goroutine managed by the provided
// start function, or directly via safego if startFn is nil.
func (mp *MessagePump) Start(ctx context.Context, startFn StartFunc) {
	safego.SetPanicHandler(func(name string, recovered any, _ []byte) {
		err := fmt.Errorf("background panic in %s: %v", name, recovered)
		mp.Enqueue(messages.Error{Err: err, Context: "background"})
	})
	if startFn != nil {
		startFn("app.external_msgs", func(ctx context.Context) error {
			return mp.Run(ctx)
		})
		return
	}
	safego.Go("app.external_msgs", func() {
		_ = mp.Run(context.Background())
	})
}

// InstallErrorHandler sets up error forwarding using the provided handler.
func (mp *MessagePump) InstallErrorHandler(handler SetErrorHandlerFunc) {
	if handler == nil {
		return
	}
	handler(func(name string, err error) {
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		mp.Enqueue(messages.Error{
			Err:     fmt.Errorf("worker %s: %w", name, err),
			Context: "supervisor:worker",
		})
	})
}

func isCriticalExternalMsg(msg tea.Msg) bool {
	if _, ok := msg.(common.CriticalExternalMsg); ok {
		return true
	}
	switch msg.(type) {
	case messages.Error, messages.SidebarPTYStopped, center.PTYStopped:
		return true
	default:
		return false
	}
}
