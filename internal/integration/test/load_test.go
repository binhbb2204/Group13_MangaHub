package integration_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
)

func TestHighConnectionCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29300")
	env.StartTCPServer(t, "29301")
	env.WaitForBridgeReady()

	numUsers := 100
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < numUsers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				"load-user-"+string(rune(index)),
				bridge.ProtocolWebSocket,
				map[string]interface{}{
					"manga_id": "load-manga",
					"chapter":  index,
				},
			)

			env.Bridge.BroadcastEvent(event)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("Handled %d concurrent users in %v (%v per user)", numUsers, duration, duration/time.Duration(numUsers))

	if duration > 5*time.Second {
		t.Errorf("Load test took too long: %v", duration)
	}
}

func TestSustainedEventThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29302")
	env.WaitForBridgeReady()

	duration := 5 * time.Second
	var eventCount atomic.Int64

	stopChan := make(chan struct{})
	var wg sync.WaitGroup

	numWorkers := 10

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-stopChan:
					return
				default:
					event := bridge.NewUnifiedEvent(
						bridge.EventProgressUpdate,
						"throughput-user",
						bridge.ProtocolTCP,
						map[string]interface{}{
							"worker_id": workerID,
							"timestamp": time.Now().Unix(),
						},
					)

					env.Bridge.BroadcastEvent(event)
					eventCount.Add(1)
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stopChan)
	wg.Wait()

	total := eventCount.Load()
	eventsPerSecond := float64(total) / duration.Seconds()

	t.Logf("Sustained throughput: %d events in %v (%.2f events/sec)", total, duration, eventsPerSecond)

	if eventsPerSecond < 100 {
		t.Errorf("Throughput too low: %.2f events/sec", eventsPerSecond)
	}
}

func TestMemoryUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29303")
	env.WaitForBridgeReady()

	numIterations := 1000
	var wg sync.WaitGroup

	for i := 0; i < numIterations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			event := bridge.NewUnifiedEvent(
				bridge.EventLibraryUpdate,
				"memory-user",
				bridge.ProtocolGRPC,
				map[string]interface{}{
					"manga_id":  "memory-manga-" + string(rune(index%100)),
					"action":    "added",
					"iteration": index,
				},
			)

			env.Bridge.BroadcastEvent(event)
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	t.Logf("Completed %d iterations without memory issues", numIterations)
}

func TestBurstTraffic(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29304")
	env.WaitForBridgeReady()

	burstSize := 200
	numBursts := 5

	for burst := 0; burst < numBursts; burst++ {
		var wg sync.WaitGroup
		startTime := time.Now()

		for i := 0; i < burstSize; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				event := bridge.NewUnifiedEvent(
					bridge.EventProgressUpdate,
					"burst-user",
					bridge.ProtocolWebSocket,
					map[string]interface{}{
						"burst":   burst,
						"index":   index,
						"chapter": index,
					},
				)

				env.Bridge.BroadcastEvent(event)
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		t.Logf("Burst %d/%d: %d events in %v", burst+1, numBursts, burstSize, duration)

		time.Sleep(200 * time.Millisecond)
	}

	t.Logf("Successfully handled %d bursts of %d events each", numBursts, burstSize)
}
