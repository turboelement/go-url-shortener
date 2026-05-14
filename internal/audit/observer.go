package audit

import (
	"io"
	"sync"
)

// Observer is notified whenever an audit event occurs.
type Observer interface {
	Notify(event AuditEvent) error
}

// Subject manages a list of observers and fans out audit events to them.
type Subject struct {
	observers []Observer
	initOnce  sync.Once
	eventChs  []chan AuditEvent
	workersWG sync.WaitGroup // tracks running worker goroutines
	eventsWG  sync.WaitGroup // tracks in-flight events being processed
}

// NewSubject creates an empty audit subject with no observers.
func NewSubject() *Subject {
	return &Subject{}
}

// Register adds an observer to receive audit events.
func (s *Subject) Register(o Observer) {
	s.observers = append(s.observers, o)
}

// NotifyAll sends an event to all registered observers asynchronously.
func (s *Subject) NotifyAll(event AuditEvent) {
	if len(s.observers) == 0 {
		return
	}

	s.initOnce.Do(s.initWorkers)
	for i := range s.eventChs {
		s.eventsWG.Add(1)
		select {
		case s.eventChs[i] <- event:
		default:
			s.eventsWG.Done()
			// events chan is full, drop event
		}
	}
}

// Flush waits until all pending events have been processed by observers.
func (s *Subject) Flush() {
	s.eventsWG.Wait()
}

// Close stops all observer workers and waits for them to finish.
func (s *Subject) Close() {
	if s.eventChs != nil {
		for _, ch := range s.eventChs {
			close(ch)
		}

		s.eventsWG.Wait()
		s.workersWG.Wait()
	}

	for _, o := range s.observers {
		if c, ok := o.(io.Closer); ok {
			c.Close()
		}
	}
}

const eventChanCap = 100

func (s *Subject) initWorkers() {
	s.eventChs = make([]chan AuditEvent, len(s.observers))
	for i, o := range s.observers {
		eventCh := make(chan AuditEvent, eventChanCap)
		s.eventChs[i] = eventCh
		s.workersWG.Add(1)
		go func(o Observer, events <-chan AuditEvent) {
			defer s.workersWG.Done()
			for event := range events {
				o.Notify(event)
				s.eventsWG.Done()
			}
		}(o, eventCh)
	}
}
