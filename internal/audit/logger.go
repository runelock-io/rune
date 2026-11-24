package audit

import "log"

// Store is the minimal interface the Logger needs — satisfied by storage.Store.
type Store interface {
	AppendAuditEvent(e Event) error
}

type Logger struct {
	store Store
}

func NewLogger(store Store) *Logger {
	return &Logger{store: store}
}

func (l *Logger) Log(event Event) {
	if err := l.store.AppendAuditEvent(event); err != nil {
		log.Printf("audit: failed to store event %s: %v", event.Type, err)
	}
}
