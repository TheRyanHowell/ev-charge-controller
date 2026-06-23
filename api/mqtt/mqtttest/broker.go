// Package mqtttest provides an embedded MQTT broker for testing.
package mqtttest

import (
	"fmt"
	"net"
	"testing"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// BrokerURL starts an embedded Mochi MQTT broker on a free port and returns
// the tcp:// address to connect to. The broker is stopped when t cleans up.
func BrokerURL(t *testing.T) string {
	t.Helper()
	port := freePort(t)

	srv := mochi.New(nil)
	// Allow all connections in tests (no auth required).
	if err := srv.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("mqtttest: add allow hook: %v", err)
	}
	addr := fmt.Sprintf(":%d", port)
	tcp := listeners.NewTCP(listeners.Config{ID: "test", Address: addr})
	if err := srv.AddListener(tcp); err != nil {
		t.Fatalf("mqtttest: add listener: %v", err)
	}
	go func() {
		_ = srv.Serve()
	}()
	t.Cleanup(func() { _ = srv.Close() })
	return fmt.Sprintf("tcp://127.0.0.1:%d", port)
}

// freePort asks the OS for an available TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mqtttest: find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
