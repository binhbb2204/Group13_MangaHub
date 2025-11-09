package metrics

import (
	"sync/atomic"
)

type Metrics struct {
	broadcastsTotal     int64
	broadcastFailsTotal int64
	activeConnections   int64
}

var global = &Metrics{}

func IncrementBroadcasts() {
	atomic.AddInt64(&global.broadcastsTotal, 1)
}

func IncrementBroadcastFails() {
	atomic.AddInt64(&global.broadcastFailsTotal, 1)
}

func SetActiveConnections(count int64) {
	atomic.StoreInt64(&global.activeConnections, count)
}

func GetBroadcasts() int64 {
	return atomic.LoadInt64(&global.broadcastsTotal)
}

func GetBroadcastFails() int64 {
	return atomic.LoadInt64(&global.broadcastFailsTotal)
}

func GetActiveConnections() int64 {
	return atomic.LoadInt64(&global.activeConnections)
}

func Reset() {
	atomic.StoreInt64(&global.broadcastsTotal, 0)
	atomic.StoreInt64(&global.broadcastFailsTotal, 0)
	atomic.StoreInt64(&global.activeConnections, 0)
}
