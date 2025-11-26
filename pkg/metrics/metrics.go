package metrics

import (
	"sync/atomic"
)

type Metrics struct {
	broadcastsTotal     int64
	broadcastFailsTotal int64
	activeConnections   int64
	messagesTotal       int64
	rateLimitedTotal    int64
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

func IncrementMessages() {
	atomic.AddInt64(&global.messagesTotal, 1)
}

func GetMessages() int64 {
	return atomic.LoadInt64(&global.messagesTotal)
}

func IncrementRateLimited() {
	atomic.AddInt64(&global.rateLimitedTotal, 1)
}

func GetRateLimited() int64 {
	return atomic.LoadInt64(&global.rateLimitedTotal)
}

func Reset() {
	atomic.StoreInt64(&global.broadcastsTotal, 0)
	atomic.StoreInt64(&global.broadcastFailsTotal, 0)
	atomic.StoreInt64(&global.activeConnections, 0)
	atomic.StoreInt64(&global.messagesTotal, 0)
	atomic.StoreInt64(&global.rateLimitedTotal, 0)
}
