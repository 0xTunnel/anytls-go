package main

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

type deadlineRecorderConn struct {
	readData  []byte
	writeData bytes.Buffer
	deadline  time.Time
}

func (c *deadlineRecorderConn) Read(b []byte) (int, error) {
	if len(c.readData) == 0 {
		return 0, io.EOF
	}
	n := copy(b, c.readData)
	c.readData = c.readData[n:]
	return n, nil
}

func (c *deadlineRecorderConn) Write(b []byte) (int, error) {
	return c.writeData.Write(b)
}

func (c *deadlineRecorderConn) Close() error { return nil }

func (c *deadlineRecorderConn) LocalAddr() net.Addr { return &net.TCPAddr{} }

func (c *deadlineRecorderConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }

func (c *deadlineRecorderConn) SetDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *deadlineRecorderConn) SetReadDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *deadlineRecorderConn) SetWriteDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

type deadlineRecorderPacketConn struct {
	readData  []byte
	writeData bytes.Buffer
	deadline  time.Time
}

func (c *deadlineRecorderPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if len(c.readData) == 0 {
		return 0, &net.UDPAddr{}, io.EOF
	}
	n := copy(b, c.readData)
	c.readData = c.readData[n:]
	return n, &net.UDPAddr{}, nil
}

func (c *deadlineRecorderPacketConn) WriteTo(b []byte, _ net.Addr) (int, error) {
	return c.writeData.Write(b)
}

func (c *deadlineRecorderPacketConn) Close() error { return nil }

func (c *deadlineRecorderPacketConn) LocalAddr() net.Addr { return &net.UDPAddr{} }

func (c *deadlineRecorderPacketConn) SetDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *deadlineRecorderPacketConn) SetReadDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *deadlineRecorderPacketConn) SetWriteDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func TestIdleTimeoutConnReadRefreshesDeadline(t *testing.T) {
	base := &deadlineRecorderConn{readData: []byte("ok")}
	wrapped := withIdleTimeoutConn(base, time.Minute)

	buffer := make([]byte, 2)
	if _, err := wrapped.Read(buffer); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if base.deadline.IsZero() {
		t.Fatal("Read() did not refresh deadline")
	}
	if !base.deadline.After(time.Now()) {
		t.Fatalf("deadline = %v, want future time", base.deadline)
	}
}

func TestIdleTimeoutConnWriteRefreshesDeadline(t *testing.T) {
	base := &deadlineRecorderConn{}
	wrapped := withIdleTimeoutConn(base, time.Minute)

	if _, err := wrapped.Write([]byte("ok")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if base.deadline.IsZero() {
		t.Fatal("Write() did not refresh deadline")
	}
	if base.writeData.String() != "ok" {
		t.Fatalf("writeData = %q", base.writeData.String())
	}
}

func TestIdleTimeoutPacketConnRefreshesDeadline(t *testing.T) {
	base := &deadlineRecorderPacketConn{readData: []byte("ok")}
	wrapped := withIdleTimeoutPacketConn(base, time.Minute)

	buffer := make([]byte, 2)
	if _, _, err := wrapped.ReadFrom(buffer); err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if base.deadline.IsZero() {
		t.Fatal("ReadFrom() did not refresh deadline")
	}

	if _, err := wrapped.WriteTo([]byte("ok"), &net.UDPAddr{}); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	if base.writeData.String() != "ok" {
		t.Fatalf("writeData = %q", base.writeData.String())
	}
}

func TestWithIdleTimeoutConnReturnsOriginalForDisabledTimeout(t *testing.T) {
	base := &deadlineRecorderConn{}
	if got := withIdleTimeoutConn(base, 0); got != base {
		t.Fatal("withIdleTimeoutConn() should return original conn when timeout is disabled")
	}
}
