package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type SyncState struct {
	ActiveConnection *ConnectionInfo `yaml:"active_connection,omitempty"`
}

type ConnectionInfo struct {
	Connected     bool      `yaml:"connected"`
	SessionID     string    `yaml:"session_id"`
	Server        string    `yaml:"server"`
	ConnectedAt   time.Time `yaml:"connected_at"`
	DeviceType    string    `yaml:"device_type"`
	DeviceName    string    `yaml:"device_name"`
	LastHeartbeat time.Time `yaml:"last_heartbeat"`
	PID           int       `yaml:"pid"`
}

var (
	syncStateMutex sync.RWMutex
	lockFile       *os.File
)

func GetSyncStatePath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "sync_state.yaml"), nil
}

func GetSyncLockPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "sync.lock"), nil
}

func LoadSyncState() (*SyncState, error) {
	syncStateMutex.RLock()
	defer syncStateMutex.RUnlock()

	statePath, err := GetSyncStatePath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return &SyncState{}, nil
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sync state: %w", err)
	}

	var state SyncState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse sync state: %w", err)
	}

	return &state, nil
}

func SaveSyncState(state *SyncState) error {
	syncStateMutex.Lock()
	defer syncStateMutex.Unlock()

	statePath, err := GetSyncStatePath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal sync state: %w", err)
	}

	tempPath := statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sync state: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to save sync state: %w", err)
	}

	return nil
}

func SetActiveConnection(sessionID, server, deviceType, deviceName string) error {
	state := &SyncState{
		ActiveConnection: &ConnectionInfo{
			Connected:     true,
			SessionID:     sessionID,
			Server:        server,
			ConnectedAt:   time.Now(),
			DeviceType:    deviceType,
			DeviceName:    deviceName,
			LastHeartbeat: time.Now(),
			PID:           os.Getpid(),
		},
	}
	return SaveSyncState(state)
}

func UpdateHeartbeat() error {
	state, err := LoadSyncState()
	if err != nil {
		return err
	}

	if state.ActiveConnection != nil {
		state.ActiveConnection.LastHeartbeat = time.Now()
		return SaveSyncState(state)
	}

	return fmt.Errorf("no active connection to update")
}

func ClearActiveConnection() error {
	state := &SyncState{
		ActiveConnection: nil,
	}
	return SaveSyncState(state)
}

func IsConnectionActive() (bool, *ConnectionInfo, error) {
	state, err := LoadSyncState()
	if err != nil {
		return false, nil, err
	}

	if state.ActiveConnection == nil || !state.ActiveConnection.Connected {
		return false, nil, nil
	}

	if !isProcessAlive(state.ActiveConnection.PID) {
		ClearActiveConnection()
		return false, nil, nil
	}

	if time.Since(state.ActiveConnection.LastHeartbeat) > 2*time.Minute {
		return false, state.ActiveConnection, nil
	}

	return true, state.ActiveConnection, nil
}

func AcquireSyncLock() error {
	lockPath, err := GetSyncLockPath()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("another sync process is already running")
		}
		return fmt.Errorf("failed to acquire sync lock: %w", err)
	}

	fmt.Fprintf(f, "%d\n", os.Getpid())
	lockFile = f

	return nil
}

func ReleaseSyncLock() error {
	if lockFile != nil {
		lockFile.Close()
		lockPath, err := GetSyncLockPath()
		if err != nil {
			return err
		}
		return os.Remove(lockPath)
	}
	return nil
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Windows, process.Signal(nil) is not supported, so we use a different approach
	// We try to send signal 0 which is a no-op on Unix, but on Windows we just check
	// if we can get the process. On Windows, FindProcess always succeeds even for
	// dead processes, so we need to try Release and check the error.
	//
	// Better approach: just check if it's the current process
	if pid == os.Getpid() {
		return true
	}

	// For other processes, try the signal approach
	// On Windows this will fail with "not supported", so we assume the process
	// is alive if FindProcess succeeded (Windows-specific behavior)
	err = process.Signal(os.Signal(nil))
	if err != nil && err.Error() == "not supported by windows" {
		// On Windows, if FindProcess succeeded, assume the process exists
		return true
	}
	return err == nil
}
