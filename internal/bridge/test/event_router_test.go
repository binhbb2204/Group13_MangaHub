package bridge_test

import (
	"os"
	"testing"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

func TestEventRouterCreation(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	router := bridge.NewEventRouter(log)

	if router == nil {
		t.Fatal("expected event router to be created")
	}
}

func TestRegisterHandler(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	router := bridge.NewEventRouter(log)

	handler := func(event bridge.UnifiedEvent) error {
		return nil
	}

	router.RegisterHandler(bridge.EventProgressUpdate, handler)

	if router.GetHandlerCount(bridge.EventProgressUpdate) != 1 {
		t.Errorf("expected 1 handler, got %d", router.GetHandlerCount(bridge.EventProgressUpdate))
	}
}

func TestRouteEvent(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	router := bridge.NewEventRouter(log)

	handlerCalled := false
	handler := func(event bridge.UnifiedEvent) error {
		handlerCalled = true
		if event.Type != bridge.EventProgressUpdate {
			t.Errorf("expected event type %s, got %s", bridge.EventProgressUpdate, event.Type)
		}
		return nil
	}

	router.RegisterHandler(bridge.EventProgressUpdate, handler)

	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"test_user",
		bridge.ProtocolTCP,
		map[string]interface{}{"test": "data"},
	)

	err := router.Route(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called")
	}
}

func TestMultipleHandlers(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	router := bridge.NewEventRouter(log)

	calledCount := 0
	handler1 := func(event bridge.UnifiedEvent) error {
		calledCount++
		return nil
	}
	handler2 := func(event bridge.UnifiedEvent) error {
		calledCount++
		return nil
	}

	router.RegisterHandler(bridge.EventLibraryUpdate, handler1)
	router.RegisterHandler(bridge.EventLibraryUpdate, handler2)

	event := bridge.NewUnifiedEvent(
		bridge.EventLibraryUpdate,
		"test_user",
		bridge.ProtocolWebSocket,
		map[string]interface{}{},
	)

	router.Route(event)

	if calledCount != 2 {
		t.Errorf("expected 2 handlers to be called, got %d", calledCount)
	}
}

func TestEventFilter(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	router := bridge.NewEventRouter(log)

	handlerCalled := false
	handler := func(event bridge.UnifiedEvent) error {
		handlerCalled = true
		return nil
	}

	filter := func(event bridge.UnifiedEvent) bool {
		return event.UserID == "allowed_user"
	}

	router.AddFilter(filter)
	router.RegisterHandler(bridge.EventProgressUpdate, handler)

	blockedEvent := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"blocked_user",
		bridge.ProtocolTCP,
		map[string]interface{}{},
	)
	router.Route(blockedEvent)

	if handlerCalled {
		t.Error("handler should not be called for filtered event")
	}

	allowedEvent := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"allowed_user",
		bridge.ProtocolTCP,
		map[string]interface{}{},
	)
	router.Route(allowedEvent)

	if !handlerCalled {
		t.Error("handler should be called for allowed event")
	}
}

func TestClearHandlers(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	router := bridge.NewEventRouter(log)

	handler := func(event bridge.UnifiedEvent) error {
		return nil
	}

	router.RegisterHandler(bridge.EventProgressUpdate, handler)
	router.RegisterHandler(bridge.EventProgressUpdate, handler)

	if router.GetHandlerCount(bridge.EventProgressUpdate) != 2 {
		t.Errorf("expected 2 handlers, got %d", router.GetHandlerCount(bridge.EventProgressUpdate))
	}

	router.ClearHandlers(bridge.EventProgressUpdate)

	if router.GetHandlerCount(bridge.EventProgressUpdate) != 0 {
		t.Errorf("expected 0 handlers after clear, got %d", router.GetHandlerCount(bridge.EventProgressUpdate))
	}
}
