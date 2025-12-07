package integration_test

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/grpc"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type TestEnvironment struct {
	Bridge       *bridge.UnifiedBridge
	TCPServer    *tcp.Server
	UDPServer    *udp.Server
	WSServer     *websocket.Server
	GRPCServer   *grpc.Server
	DB           *sql.DB
	Logger       *logger.Logger
	cleanup      []func()
	cleanupMutex sync.Mutex
}

func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	logger.Init(logger.ERROR, false, nil)
	log := logger.GetLogger()

	dbPath := fmt.Sprintf("./test_data/integration_test_%d.db", time.Now().Unix())
	os.MkdirAll("./test_data", 0755)

	if err := database.InitDatabase(dbPath); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	unifiedBridge := bridge.NewUnifiedBridge(log)
	unifiedBridge.Start()

	env := &TestEnvironment{
		Bridge: unifiedBridge,
		DB:     database.DB,
		Logger: log,
		cleanup: []func(){
			func() { os.Remove(dbPath) },
			func() { database.Close() },
			func() { unifiedBridge.Stop() },
		},
	}

	return env
}

func (env *TestEnvironment) StartTCPServer(t *testing.T, port string) {
	env.TCPServer = tcp.NewServer(port, nil)
	if err := env.TCPServer.Start(); err != nil {
		t.Fatalf("Failed to start TCP server: %v", err)
	}
	env.AddCleanup(func() { env.TCPServer.Stop() })
}

func (env *TestEnvironment) StartUDPServer(t *testing.T, port string) {
	env.UDPServer = udp.NewServer(port)
	env.UDPServer.SetBridge(env.Bridge)
	if err := env.UDPServer.Start(); err != nil {
		t.Fatalf("Failed to start UDP server: %v", err)
	}
	env.AddCleanup(func() { env.UDPServer.Stop() })
}

func (env *TestEnvironment) StartWebSocketServer(t *testing.T, jwtSecret string) {
	env.WSServer = websocket.NewServer(env.DB, jwtSecret)
	env.WSServer.SetBridge(env.Bridge)
	env.AddCleanup(func() {})
}

func (env *TestEnvironment) StartGRPCServer(t *testing.T) {
	env.GRPCServer = grpc.NewServer(env.DB)
	env.GRPCServer.SetBridge(env.Bridge)
	env.AddCleanup(func() {})
}

func (env *TestEnvironment) AddCleanup(fn func()) {
	env.cleanupMutex.Lock()
	defer env.cleanupMutex.Unlock()
	env.cleanup = append(env.cleanup, fn)
}

func (env *TestEnvironment) Cleanup() {
	env.cleanupMutex.Lock()
	defer env.cleanupMutex.Unlock()

	for i := len(env.cleanup) - 1; i >= 0; i-- {
		env.cleanup[i]()
	}
}

func (env *TestEnvironment) WaitForBridgeReady() {
	time.Sleep(100 * time.Millisecond)
}
