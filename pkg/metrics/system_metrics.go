package metrics

import (
	"sync/atomic"
	"time"
)

type SystemMetrics struct {
	EventsProcessed       atomic.Int64
	EventsFailed          atomic.Int64
	AverageLatency        atomic.Int64
	PeakLatency           atomic.Int64
	ActiveConnections     atomic.Int64
	TotalBytesTransferred atomic.Int64
	ErrorRate             atomic.Int64
	lastResetTime         time.Time
}

var systemMetrics = &SystemMetrics{
	lastResetTime: time.Now(),
}

func RecordEventProcessed() {
	systemMetrics.EventsProcessed.Add(1)
}

func RecordEventFailed() {
	systemMetrics.EventsFailed.Add(1)
	total := systemMetrics.EventsProcessed.Load()
	failed := systemMetrics.EventsFailed.Load()
	if total > 0 {
		errorRate := (failed * 100) / total
		systemMetrics.ErrorRate.Store(errorRate)
	}
}

func RecordLatency(latencyMs int64) {
	current := systemMetrics.AverageLatency.Load()
	processed := systemMetrics.EventsProcessed.Load()

	if processed > 0 {
		newAvg := (current*(processed-1) + latencyMs) / processed
		systemMetrics.AverageLatency.Store(newAvg)
	}

	peak := systemMetrics.PeakLatency.Load()
	if latencyMs > peak {
		systemMetrics.PeakLatency.Store(latencyMs)
	}
}

func RecordBytesTransferred(bytes int64) {
	systemMetrics.TotalBytesTransferred.Add(bytes)
}

func GetSystemMetrics() map[string]int64 {
	return map[string]int64{
		"events_processed":   systemMetrics.EventsProcessed.Load(),
		"events_failed":      systemMetrics.EventsFailed.Load(),
		"average_latency":    systemMetrics.AverageLatency.Load(),
		"peak_latency":       systemMetrics.PeakLatency.Load(),
		"active_connections": systemMetrics.ActiveConnections.Load(),
		"bytes_transferred":  systemMetrics.TotalBytesTransferred.Load(),
		"error_rate":         systemMetrics.ErrorRate.Load(),
	}
}

func ResetMetrics() {
	systemMetrics.EventsProcessed.Store(0)
	systemMetrics.EventsFailed.Store(0)
	systemMetrics.AverageLatency.Store(0)
	systemMetrics.PeakLatency.Store(0)
	systemMetrics.ErrorRate.Store(0)
	systemMetrics.lastResetTime = time.Now()
}

func GetUptime() time.Duration {
	return time.Since(systemMetrics.lastResetTime)
}
