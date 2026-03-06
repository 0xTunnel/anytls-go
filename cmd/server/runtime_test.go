package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestWaitForNextTickStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	startedAt := time.Now()
	if waitForNextTick(ctx, time.Second) {
		t.Fatal("waitForNextTick() = true, want false")
	}
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("waitForNextTick() returned too slowly: %v", elapsed)
	}
}

func TestServerRuntimeShutdownClosesTrackedConnections(t *testing.T) {
	listener := &stubListener{}
	runtime := newServerRuntime(nil, listener)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	runtime.trackConn(serverConn)
	runtime.connWG.Add(1)
	readDone := make(chan struct{})
	go func() {
		defer runtime.connWG.Done()
		defer close(readDone)
		var buf [1]byte
		_, _ = serverConn.Read(buf[:])
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if !listener.closed {
		t.Fatal("Shutdown() did not close listener")
	}

	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("tracked connection was not closed during shutdown")
	}
}

func TestServerRuntimeServeReturnsNilAfterShutdown(t *testing.T) {
	listener := newBlockingListener()
	runtime := newServerRuntime(&myServer{}, listener)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runtime.Serve(ctx)
	}()

	cancel()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve() did not stop after listener close")
	}
}

func TestShouldRetryAccept(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable op error",
			err:  &net.OpError{Err: syscall.EMFILE},
			want: true,
		},
		{
			name: "wrapped retryable op error",
			err:  &net.OpError{Err: fmt.Errorf("wrap: %w", syscall.ENOMEM)},
			want: true,
		},
		{
			name: "non retryable op error",
			err:  &net.OpError{Err: syscall.EINVAL},
			want: false,
		},
		{
			name: "non op error",
			err:  errors.New("boom"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryAccept(tt.err); got != tt.want {
				t.Fatalf("shouldRetryAccept() = %v, want %v", got, tt.want)
			}
		})
	}
}

type stubListener struct {
	closed bool
}

func (l *stubListener) Accept() (net.Conn, error) {
	return nil, net.ErrClosed
}

func (l *stubListener) Close() error {
	l.closed = true
	return nil
}

func (l *stubListener) Addr() net.Addr {
	return stubAddr("stub")
}

type blockingListener struct {
	closed chan struct{}
	once   chan struct{}
}

func newBlockingListener() *blockingListener {
	return &blockingListener{
		closed: make(chan struct{}),
		once:   make(chan struct{}, 1),
	}
}

func (l *blockingListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, net.ErrClosed
}

func (l *blockingListener) Close() error {
	select {
	case <-l.closed:
		return nil
	default:
		close(l.closed)
		return nil
	}
}

func (l *blockingListener) Addr() net.Addr {
	return stubAddr("blocking")
}

type stubAddr string

func (a stubAddr) Network() string {
	return string(a)
}

func (a stubAddr) String() string {
	return string(a)
}
