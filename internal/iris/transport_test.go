package iris

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testService(t *testing.T, handle func(context.Context, request) response) (config, func()) {
	t.Helper()

	return testServiceLimit(t, handle, 128)
}

func testServiceLimit(t *testing.T, handle func(context.Context, request) response, maximum int) (config, func()) {
	t.Helper()

	s, cfg := testStore(t)
	if err := privateDirectory(filepath.Dir(cfg.runtimeRoot)); err != nil {
		t.Fatal(err)
	}

	cfg.settings.MaxConnections = maximum

	lock, err := acquireRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}

	listener, err := listenUnix(cfg.socket)
	if err != nil {
		t.Fatal(err)
	}

	if handle == nil {
		handle = s.handle
	}

	service := server{config: cfg, handle: handle}
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- service.serve(ctx, listener) }()

	var once sync.Once

	stop := func() {
		once.Do(func() {
			cancel()

			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(3 * time.Second):
				t.Error("service did not stop")
			}

			if err := lock.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	t.Cleanup(stop)

	return cfg, stop
}

func statsRequest() request {
	return request{Protocol: protocolVersion, Operation: "stats", Params: json.RawMessage(`{"all_projects":true}`)}
}

// A real protected Unix socket returns versioned storage readiness without opening HTTP.
func TestServiceRoundTrip(t *testing.T) {
	cfg, _ := testService(t, nil)
	if err := checkSocket(cfg.socket); err != nil {
		t.Fatal(err)
	}

	reply, err := callService(t.Context(), cfg, statsRequest())
	if err != nil {
		t.Fatal(err)
	}

	var stats statistics
	if err := decodeJSON(reply.Result, &stats); err != nil {
		t.Fatal(err)
	}

	if !stats.Ready || stats.StorageSchemaVersion != 1 || stats.Projects != 0 {
		t.Fatalf("statistics=%+v", stats)
	}
}

// Invalid frames, protocol versions, fields and scope fail with bounded stable errors.
func TestServiceInvalidRequests(t *testing.T) {
	cfg, _ := testService(t, nil)

	cases := []struct{ name, data, code string }{
		{"malformed", "{\n", "invalid_request"},
		{"utf8", "{\"x\":\"\xff\"}\n", "invalid_request"},
		{"trailing", "{} {}\n", "invalid_request"},
		{"two frames", "{}\n{}\n", "invalid_request"},
		{"unknown field", "{\"protocol\":\"iris/v1\",\"operation\":\"stats\",\"params\":{},\"extra\":1}\n", "invalid_request"},
		{"version", "{\"protocol\":\"other/v1\",\"operation\":\"stats\",\"params\":{}}\n", "unsupported_protocol"},
		{"operation", "{\"protocol\":\"iris/v1\",\"operation\":\"sql\",\"params\":{}}\n", "unknown_operation"},
		{"params", "{\"protocol\":\"iris/v1\",\"operation\":\"stats\",\"params\":{\"extra\":1}}\n", "invalid_request"},
		{"scope", "{\"protocol\":\"iris/v1\",\"operation\":\"stats\",\"params\":{}}\n", "project_not_found"},
		{"oversized", strings.Repeat("x", maxRequestBytes) + "\n", "limit_exceeded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reply := rawRequest(t, cfg.socket, tc.data)
			if reply.OK || reply.Error == nil || reply.Error.Code != tc.code {
				t.Fatalf("response=%+v", reply)
			}
		})
	}
}

func rawRequest(t *testing.T, path, data string) response {
	t.Helper()

	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
	}()

	if err := connection.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}

	if _, err := connection.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}

	if err := connection.CloseWrite(); err != nil {
		t.Fatal(err)
	}

	frame, err := readFrame(connection, maxResponseBytes)
	if err != nil {
		t.Fatal(err)
	}

	var reply response
	if err := decodeJSON(frame, &reply); err != nil {
		t.Fatal(err)
	}

	return reply
}

// A response exceeding its wire budget is replaced before any partial result is sent.
func TestServiceResponseLimit(t *testing.T) {
	cfg, _ := testService(t, func(context.Context, request) response {
		return success(map[string]string{"body": strings.Repeat("x", maxResponseBytes)})
	})
	_, err := callService(t.Context(), cfg, statsRequest())

	var service *serviceError
	if !errors.As(err, &service) || service.Code != "limit_exceeded" {
		t.Fatalf("error=%v", err)
	}
}

// Shutdown closes owned blocked connections, waits for workers and removes the socket.
func TestServiceShutdown(t *testing.T) {
	cfg, stop := testService(t, nil)

	connection, err := net.Dial("unix", cfg.socket)
	if err != nil {
		t.Fatal(err)
	}

	stop()

	if _, err := os.Stat(cfg.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains: %v", err)
	}

	if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	if _, err := connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("blocked connection remained open")
	}

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
}

// Cancelling a client call returns promptly without terminating the daemon.
func TestClientCancellation(t *testing.T) {
	entered := make(chan struct{})
	cfg, _ := testService(t, func(ctx context.Context, _ request) response {
		close(entered)
		<-ctx.Done()

		return failure("cancelled", "cancelled", false)
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { _, err := callService(ctx, cfg, statsRequest()); done <- err }()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request did not arrive")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client cancellation blocked")
	}
}

// A disconnected peer cannot kill the daemon or prevent a later valid request.
func TestServiceDisconnect(t *testing.T) {
	cfg, _ := testService(t, nil)

	connection, err := net.Dial("unix", cfg.socket)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := connection.Write([]byte("{\"protocol\":")); err != nil {
		t.Fatal(err)
	}

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := callService(t.Context(), cfg, statsRequest()); err != nil {
		t.Fatal(err)
	}
}

// Saturation returns retryable backpressure without creating unbounded workers.
func TestServiceConnectionLimit(t *testing.T) {
	entered := make(chan struct{})
	cfg, stop := testServiceLimit(t, func(ctx context.Context, _ request) response {
		close(entered)
		<-ctx.Done()

		return failure("cancelled", "cancelled", false)
	}, 1)
	done := make(chan error, 1)

	go func() { _, err := callService(t.Context(), cfg, statsRequest()); done <- err }()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first request did not enter")
	}

	_, err := callService(t.Context(), cfg, statsRequest())

	var service *serviceError
	if !errors.As(err, &service) || service.Code != "server_busy" || !service.Retryable {
		t.Fatalf("backpressure error=%v", err)
	}

	stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("owned request did not stop")
	}
}
