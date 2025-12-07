package metrics

import (
	"sync/atomic"
)

var (
	tcpConnections       int64
	udpConnections       int64
	websocketConnections int64
	grpcConnections      int64
)

func IncrementConnectionCount(protocol string) {
	switch protocol {
	case "tcp":
		atomic.AddInt64(&tcpConnections, 1)
	case "udp":
		atomic.AddInt64(&udpConnections, 1)
	case "websocket":
		atomic.AddInt64(&websocketConnections, 1)
	case "grpc":
		atomic.AddInt64(&grpcConnections, 1)
	}
}

func DecrementConnectionCount(protocol string) {
	switch protocol {
	case "tcp":
		atomic.AddInt64(&tcpConnections, -1)
	case "udp":
		atomic.AddInt64(&udpConnections, -1)
	case "websocket":
		atomic.AddInt64(&websocketConnections, -1)
	case "grpc":
		atomic.AddInt64(&grpcConnections, -1)
	}
}

func GetTCPConnections() int64 {
	return atomic.LoadInt64(&tcpConnections)
}

func GetUDPConnections() int64 {
	return atomic.LoadInt64(&udpConnections)
}

func GetWebSocketConnections() int64 {
	return atomic.LoadInt64(&websocketConnections)
}

func GetGRPCConnections() int64 {
	return atomic.LoadInt64(&grpcConnections)
}
