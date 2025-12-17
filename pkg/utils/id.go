package utils

import (
	"crypto/rand"
	"encoding/hex"
	"net"
)

func GenerateID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GetLocalIP detects the local IP address by dialing to an external address
// Returns the IP that would be used for outbound connections
func GetLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
