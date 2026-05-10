package audit

import "sync"

type Observer interface {
	Notify(event AuditEvent) error
}

type Subject struct {
	observers []Observer
	initOnce  sync.Once
	eventChs  []chan AuditEvent
}

func NewSubject() *Subject {
	return &Subject{}
}

func (s *Subject) Register(o Observer) {
	s.observers = append(s.observers, o)
}

func (s *Subject) NotifyAll(event AuditEvent) {
	if len(s.observers) == 0 {
		return
	}

	s.initOnce.Do(s.initWorkers)
	for i := range s.eventChs {
		select {
		case s.eventChs[i] <- event:
		default:
			// events chan is full, drop event
		}
	}
}

func (s *Subject) Close() {
	if s.eventChs == nil {
		return
	}

	for _, ch := range s.eventChs {
		close(ch)
	}
}

const eventChanCap = 100

func (s *Subject) initWorkers() {
	s.eventChs = make([]chan AuditEvent, len(s.observers))
	for i, o := range s.observers {
		eventCh := make(chan AuditEvent, eventChanCap)
		s.eventChs[i] = eventCh
		go func(o Observer, events <-chan AuditEvent) {
			for event := range events {
				o.Notify(event)
			}
		}(o, eventCh)
	}
}
