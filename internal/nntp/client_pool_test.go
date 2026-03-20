package nntp

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
)

func newTestClientForProvider(t *testing.T, addr string, maxConnections int, retries int) (*Client, config.UsenetProvider) {
	t.Helper()
	config.SetConfigPath(t.TempDir())

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	provider := config.UsenetProvider{
		Host:           host,
		Port:           port,
		MaxConnections: maxConnections,
		Priority:       1,
	}

	cfg := &config.Config{
		Retries: retries,
		Usenet: config.Usenet{
			Providers: []config.UsenetProvider{provider},
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	return client, provider
}

func TestClient_ConnectionPoolReuse(t *testing.T) {
	var acceptedConnections atomic.Int32
	var dateCommands atomic.Int32

	srv := newFakeServer(t, func(c net.Conn) {
		acceptedConnections.Add(1)

		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			if strings.HasPrefix(cmd, "DATE") {
				dateCommands.Add(1)
				reply(w, "111 20260312060000")
			}
		}
	})
	t.Cleanup(srv.close)

	client, _ := newTestClientForProvider(t, srv.addr(), 2, 0)

	for i := 0; i < 5; i++ {
		if err := client.ExecuteWithFailover(context.Background(), func(conn *Connection) error {
			return conn.ping()
		}); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}

	if got := acceptedConnections.Load(); got != 1 {
		t.Fatalf("accepted connections = %d, want 1 (connection reuse)", got)
	}
	if got := dateCommands.Load(); got != 5 {
		t.Fatalf("DATE commands = %d, want 5", got)
	}
}

func TestClient_ConnectionExhaustion(t *testing.T) {
	var acceptedConnections atomic.Int32

	srv := newFakeServer(t, func(c net.Conn) {
		acceptedConnections.Add(1)

		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "DATE") {
				reply(w, "111 20260312060000")
			}
		}
	})
	t.Cleanup(srv.close)

	client, provider := newTestClientForProvider(t, srv.addr(), 1, 0)

	firstConn, _, err := client.getConnectionFromProvider(context.Background(), provider)
	if err != nil {
		t.Fatalf("acquire first connection: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _, err = client.getConnectionFromProvider(waitCtx, provider)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded while pool exhausted, got: %v", err)
	}

	if got := acceptedConnections.Load(); got != 1 {
		t.Fatalf("accepted connections = %d, want 1 while exhausted", got)
	}

	client.put(firstConn, provider)

	secondConn, _, err := client.getConnectionFromProvider(context.Background(), provider)
	if err != nil {
		t.Fatalf("acquire after releasing slot: %v", err)
	}
	client.put(secondConn, provider)

	if got := acceptedConnections.Load(); got != 1 {
		t.Fatalf("accepted connections after reuse = %d, want 1", got)
	}
}

func TestClient_ReconnectAfterConnectionFailure(t *testing.T) {
	var acceptedConnections atomic.Int32
	var dateCommands atomic.Int32

	srv := newFakeServer(t, func(c net.Conn) {
		connID := acceptedConnections.Add(1)

		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			if strings.HasPrefix(cmd, "DATE") {
				dateCommands.Add(1)
				if connID == 1 {
					_ = c.Close() // Simulate dropped connection on first attempt.
					return
				}
				reply(w, "111 20260312060000")
			}
		}
	})
	t.Cleanup(srv.close)

	client, _ := newTestClientForProvider(t, srv.addr(), 1, 1)

	err := client.ExecuteWithFailover(context.Background(), func(conn *Connection) error {
		return conn.ping()
	})
	if err != nil {
		t.Fatalf("operation should succeed after reconnect, got: %v", err)
	}

	if got := acceptedConnections.Load(); got != 2 {
		t.Fatalf("accepted connections = %d, want 2 (reconnect expected)", got)
	}
	if got := dateCommands.Load(); got != 2 {
		t.Fatalf("DATE commands = %d, want 2 (first failed, second succeeded)", got)
	}
}
