package main

import (
	"net"
	"time"
)

type idleTimeoutConn struct {
	net.Conn
	timeout time.Duration
}

func withIdleTimeoutConn(conn net.Conn, timeout time.Duration) net.Conn {
	if conn == nil || timeout <= 0 {
		return conn
	}
	return &idleTimeoutConn{Conn: conn, timeout: timeout}
}

func (c *idleTimeoutConn) Read(b []byte) (int, error) {
	if err := c.Conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}

func (c *idleTimeoutConn) Write(b []byte) (int, error) {
	if err := c.Conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(b)
}

type idleTimeoutPacketConn struct {
	net.PacketConn
	timeout time.Duration
}

func withIdleTimeoutPacketConn(conn net.PacketConn, timeout time.Duration) net.PacketConn {
	if conn == nil || timeout <= 0 {
		return conn
	}
	return &idleTimeoutPacketConn{PacketConn: conn, timeout: timeout}
}

func (c *idleTimeoutPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if err := c.PacketConn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, nil, err
	}
	return c.PacketConn.ReadFrom(b)
}

func (c *idleTimeoutPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	if err := c.PacketConn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.PacketConn.WriteTo(b, addr)
}
