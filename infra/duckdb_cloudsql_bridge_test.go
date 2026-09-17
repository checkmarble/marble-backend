package infra

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestCloudSqlBridgeCloseUnblocksActiveConnection(t *testing.T) {
	dialed := make(chan net.Conn, 1)
	bridge, err := startCloudSqlBridge(t.Context(), "instance", func(context.Context, string) (net.Conn, error) {
		bridgeConn, upstreamConn := net.Pipe()
		dialed <- upstreamConn
		return bridgeConn, nil
	})
	if err != nil {
		t.Fatalf("start bridge: %v", err)
	}

	client, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		bridge.Close()
		t.Fatalf("connect to bridge: %v", err)
	}
	defer client.Close()

	upstream := waitForValue(t, dialed)
	defer upstream.Close()

	closed := make(chan struct{})
	go func() {
		bridge.Close()
		close(closed)
	}()

	waitForSignal(t, closed)
}

func TestCloudSqlBridgeForwardsBidirectionally(t *testing.T) {
	dialed := make(chan net.Conn, 1)
	bridge, err := startCloudSqlBridge(t.Context(), "instance", func(context.Context, string) (net.Conn, error) {
		bridgeConn, upstreamConn := net.Pipe()
		dialed <- upstreamConn
		return bridgeConn, nil
	})
	if err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	client, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		t.Fatalf("connect to bridge: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}

	upstream := waitForValue(t, dialed)
	defer upstream.Close()
	if err := upstream.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set upstream deadline: %v", err)
	}

	if _, err := client.Write([]byte("request")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	request := make([]byte, len("request"))
	if _, err := io.ReadFull(upstream, request); err != nil {
		t.Fatalf("read request: %v", err)
	}
	if string(request) != "request" {
		t.Fatalf("unexpected request: %q", request)
	}

	if _, err := upstream.Write([]byte("response")); err != nil {
		t.Fatalf("write response: %v", err)
	}
	response := make([]byte, len("response"))
	if _, err := io.ReadFull(client, response); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(response) != "response" {
		t.Fatalf("unexpected response: %q", response)
	}
}

func TestCloudSqlBridgeParentCancellationStopsBridge(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	bridge, err := startCloudSqlBridge(ctx, "instance", func(context.Context, string) (net.Conn, error) {
		t.Fatal("unexpected dial")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("start bridge: %v", err)
	}

	stopped := make(chan struct{})
	go func() {
		bridge.wg.Wait()
		close(stopped)
	}()

	cancel()
	waitForSignal(t, stopped)
	bridge.Close()
}

func TestCloudSqlBridgeCloseIsConcurrentSafe(t *testing.T) {
	bridge, err := startCloudSqlBridge(t.Context(), "instance", func(context.Context, string) (net.Conn, error) {
		t.Fatal("unexpected dial")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("start bridge: %v", err)
	}

	var callers sync.WaitGroup
	callers.Add(5)
	for range 5 {
		go func() {
			defer callers.Done()
			bridge.Close()
		}()
	}

	closed := make(chan struct{})
	go func() {
		callers.Wait()
		close(closed)
	}()
	waitForSignal(t, closed)
}

func waitForValue[T any](t *testing.T, values <-chan T) T {
	t.Helper()

	select {
	case value := <-values:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for value")
		var zero T
		return zero
	}
}

func waitForSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal")
	}
}
