package main

import (
	"anytls/internal/node/state"
	"anytls/proxy/padding"
	"anytls/proxy/session"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sirupsen/logrus"
)

func handleTcpConnection(ctx context.Context, c net.Conn, s *myServer) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorln("[BUG]", r, string(debug.Stack()))
		}
	}()

	c = tls.Server(c, s.tlsConfig)
	defer c.Close()

	b := buf.NewPacket()
	defer b.Release()

	n, err := b.ReadOnceFrom(c)
	if err != nil {
		logrus.Debugln("ReadOnceFrom:", err)
		return
	}
	c = bufio.NewCachedConn(c, b)

	authHash, err := b.ReadBytes(32)
	if err != nil {
		b.Resize(0, n)
		fallback(ctx, c)
		return
	}
	var userID int64
	var releaseOnce sync.Once
	var release func()
	defer func() {
		releaseOnce.Do(func() {
			if release != nil {
				release()
			}
		})
	}()
	if s.IsNodeMode() {
		snapshot := s.snapshotStore.Load()
		if snapshot == nil {
			logrus.Errorln("node snapshot is empty")
			b.Resize(0, n)
			fallback(ctx, c)
			return
		}
		user, ok := snapshot.LookupAuthHash(authHash)
		if !ok {
			b.Resize(0, n)
			fallback(ctx, c)
			return
		}
		remoteIP := remoteIPFromAddr(c.RemoteAddr())
		if err := s.deviceTracker.Acquire(user.ID, remoteIP, user.DeviceLimit); err != nil {
			if errors.Is(err, state.ErrDeviceLimitExceeded) {
				logrus.Warnf("reject user %d from %s: device limit exceeded", user.ID, remoteIP)
			} else {
				logrus.Warnf("reject user %d from %s: %v", user.ID, remoteIP, err)
			}
			b.Resize(0, n)
			fallback(ctx, c)
			return
		}
		userID = user.ID
		release = func() {
			s.deviceTracker.Release(user.ID, remoteIP)
		}
	}
	by, err := b.ReadBytes(2)
	if err != nil {
		b.Resize(0, n)
		fallback(ctx, c)
		return
	}
	paddingLen := binary.BigEndian.Uint16(by)
	if paddingLen > 0 {
		_, err = b.ReadBytes(int(paddingLen))
		if err != nil {
			b.Resize(0, n)
			fallback(ctx, c)
			return
		}
	}

	session := session.NewServerSession(c, func(stream *session.Stream) {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorln("[BUG]", r, string(debug.Stack()))
			}
		}()
		defer stream.Close()

		destination, err := M.SocksaddrSerializer.ReadAddrPort(stream)
		if err != nil {
			logrus.Debugln("ReadAddrPort:", err)
			return
		}

		if strings.Contains(destination.String(), "udp-over-tcp.arpa") {
			proxyOutboundUoT(ctx, stream, destination)
		} else {
			proxyOutboundTCP(ctx, stream, destination)
		}
	}, &padding.DefaultPaddingFactory)
	if userID > 0 {
		session.SetUserContext(userID, s.traffic)
		session.SetCloseHook(func() {
			releaseOnce.Do(func() {
				if release != nil {
					release()
				}
			})
		})
	}
	session.Run()
	session.Close()
}

func remoteIPFromAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func fallback(ctx context.Context, c net.Conn) {
	// 暂未实现
	logrus.Debugln("fallback:", fmt.Sprint(c.RemoteAddr()))
}
