package bridge

import (
	"sync"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type EventHandler func(event UnifiedEvent) error

type EventFilter func(event UnifiedEvent) bool

type EventRouter struct {
	logger   *logger.Logger
	handlers map[EventType][]EventHandler
	filters  []EventFilter
	mu       sync.RWMutex
}

func NewEventRouter(log *logger.Logger) *EventRouter {
	return &EventRouter{
		logger:   log,
		handlers: make(map[EventType][]EventHandler),
		filters:  make([]EventFilter, 0),
	}
}

func (r *EventRouter) RegisterHandler(eventType EventType, handler EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[eventType] = append(r.handlers[eventType], handler)
	r.logger.Info("event_handler_registered", "event_type", eventType)
}

func (r *EventRouter) AddFilter(filter EventFilter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.filters = append(r.filters, filter)
}

func (r *EventRouter) Route(event UnifiedEvent) error {
	if !r.shouldProcess(event) {
		r.logger.Debug("event_filtered", "event_id", event.ID, "type", event.Type)
		return nil
	}

	r.mu.RLock()
	handlers := r.handlers[event.Type]
	r.mu.RUnlock()

	if len(handlers) == 0 {
		r.logger.Debug("no_handlers_for_event", "event_type", event.Type)
		return nil
	}

	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			if err := h(event); err != nil {
				r.logger.Error("handler_error",
					"event_id", event.ID,
					"event_type", event.Type,
					"error", err.Error())
			}
		}(handler)
	}
	wg.Wait()

	r.logger.Debug("event_routed",
		"event_id", event.ID,
		"event_type", event.Type,
		"handlers_count", len(handlers))

	return nil
}

func (r *EventRouter) shouldProcess(event UnifiedEvent) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, filter := range r.filters {
		if !filter(event) {
			return false
		}
	}
	return true
}

func (r *EventRouter) GetHandlerCount(eventType EventType) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers[eventType])
}

func (r *EventRouter) ClearHandlers(eventType EventType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, eventType)
	r.logger.Info("handlers_cleared", "event_type", eventType)
}
