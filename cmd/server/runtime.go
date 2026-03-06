package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type serverRuntime struct {
	server   *myServer
	listener net.Listener

	bgWG   sync.WaitGroup
	connWG sync.WaitGroup

	connMu      sync.Mutex
	activeConns map[net.Conn]struct{}

	shutdownOnce sync.Once
}

func newServerRuntime(server *myServer, listener net.Listener) *serverRuntime {
	return &serverRuntime{
		server:      server,
		listener:    listener,
		activeConns: make(map[net.Conn]struct{}),
	}
}

func (r *serverRuntime) StartBackgroundTasks(ctx context.Context) {
	if r == nil || r.server == nil || !r.server.IsNodeMode() {
		return
	}
	r.bgWG.Add(2)
	go func() {
		defer r.bgWG.Done()
		r.server.runSyncLoop(ctx)
	}()
	go func() {
		defer r.bgWG.Done()
		r.server.runReportLoop(ctx)
	}()
}

func (r *serverRuntime) Serve(ctx context.Context) error {
	if r == nil || r.listener == nil {
		return fmt.Errorf("listener is required")
	}
	retryDelay := 100 * time.Millisecond
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			if ctx.Err() != nil && isListenerClosedError(err) {
				return nil
			}
			if shouldRetryAccept(err) {
				eventLogger("runtime", logrus.Fields{"retry_delay": retryDelay.String()}, "accept_retryable_error").WithError(err).Warn("accept failed with retryable error")
				if !waitForNextTick(ctx, retryDelay) {
					return nil
				}
				retryDelay *= 2
				if retryDelay > time.Second {
					retryDelay = time.Second
				}
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept tcp connection: %w", err)
		}
		retryDelay = 100 * time.Millisecond

		r.trackConn(conn)
		r.connWG.Add(1)
		go func() {
			defer r.connWG.Done()
			defer r.untrackConn(conn)
			handleTcpConnection(ctx, conn, r.server)
		}()
	}
}

func (r *serverRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.shutdownOnce.Do(func() {
		if r.listener != nil {
			if err := r.listener.Close(); err != nil && !isListenerClosedError(err) {
				eventLogger("runtime", nil, "close_listener_failed").WithError(err).Debug("close listener returned error")
			}
		}
		for _, conn := range r.snapshotActiveConns() {
			_ = conn.Close()
		}
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.bgWG.Wait()
		r.connWG.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for shutdown: %w", ctx.Err())
	}
}

func (r *serverRuntime) trackConn(conn net.Conn) {
	if r == nil || conn == nil {
		return
	}
	r.connMu.Lock()
	defer r.connMu.Unlock()
	r.activeConns[conn] = struct{}{}
}

func (r *serverRuntime) untrackConn(conn net.Conn) {
	if r == nil || conn == nil {
		return
	}
	r.connMu.Lock()
	defer r.connMu.Unlock()
	delete(r.activeConns, conn)
}

func (r *serverRuntime) snapshotActiveConns() []net.Conn {
	if r == nil {
		return nil
	}
	r.connMu.Lock()
	defer r.connMu.Unlock()
	conns := make([]net.Conn, 0, len(r.activeConns))
	for conn := range r.activeConns {
		conns = append(conns, conn)
	}
	return conns
}

func isListenerClosedError(err error) bool {
	return err != nil && (errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "closed network connection"))
}

func shouldRetryAccept(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	return errors.Is(opErr.Err, syscall.EINTR) ||
		errors.Is(opErr.Err, syscall.ECONNABORTED) ||
		errors.Is(opErr.Err, syscall.EMFILE) ||
		errors.Is(opErr.Err, syscall.ENFILE) ||
		errors.Is(opErr.Err, syscall.ENOBUFS) ||
		errors.Is(opErr.Err, syscall.ENOMEM)
}
