//go:build !darwin

package notify

import "time"

// stub posts nothing; the per-OS notifiers replace it.
type stub struct{}

func (stub) Send(Request) bool                          { return false }
func (stub) Ask(Request, string, time.Duration) *Answer { return nil }

// New returns the notifier for the current OS.
func New() Notifier { return stub{} }
