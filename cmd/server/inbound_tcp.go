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
	remoteAddr := ""
	remoteIP := ""
	if c != nil && c.RemoteAddr() != nil {
		remoteAddr = c.RemoteAddr().String()
		remoteIP = remoteIPFromAddr(c.RemoteAddr())
	}
	connTag := ""
	entry := eventLogger("inbound", logrus.Fields{
		"remote_addr": remoteAddr,
		"remote_ip":   remoteIP,
	}, "connection_started")
	entry.WithField("event", "connection_start").Debug(formatConnectionLifecycleMessage("", remoteAddr, "start"))
	defer func() {
		if r := recover(); r != nil {
			entry.WithField("event", "panic").WithField("panic", r).Errorln(string(debug.Stack()))
		}
	}()

	c = tls.Server(c, s.tlsConfig)
	defer c.Close()

	b := buf.NewPacket()
	defer b.Release()

	n, err := b.ReadOnceFrom(c)
	if err != nil {
		entry.WithField("event", "initial_read_failed").WithError(err).Debug("failed to read initial payload")
		return
	}
	c = bufio.NewCachedConn(c, b)

	authHash, err := b.ReadBytes(32)
	if err != nil {
		b.Resize(0, n)
		fallback(ctx, c, entry, "missing_auth_hash")
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
			entry.WithField("event", "missing_snapshot").Error("node snapshot is empty")
			b.Resize(0, n)
			fallback(ctx, c, entry, "missing_snapshot")
			return
		}
		user, ok := snapshot.LookupAuthHash(authHash)
		if !ok {
			b.Resize(0, n)
			fallback(ctx, c, entry, "auth_not_matched")
			return
		}
		if err := s.deviceTracker.Acquire(user.ID, remoteIP, user.DeviceLimit); err != nil {
			rejectEntry := entry.WithFields(logrus.Fields{"user_id": user.ID, "device_limit": user.DeviceLimit})
			if errors.Is(err, state.ErrDeviceLimitExceeded) {
				rejectEntry.WithField("event", "device_reject").Warn("device limit exceeded")
			} else {
				rejectEntry.WithField("event", "device_reject").WithError(err).Warn("reject user due to tracker error")
			}
			b.Resize(0, n)
			fallback(ctx, c, rejectEntry, "device_reject")
			return
		}
		userID = user.ID
		connTag = buildConnectionTag(userID, remoteAddr)
		entry = entry.WithFields(logrus.Fields{"user_id": user.ID, "conn_tag": connTag})
		release = func() {
			s.deviceTracker.Release(user.ID, remoteIP)
		}
	}
	by, err := b.ReadBytes(2)
	if err != nil {
		b.Resize(0, n)
		fallback(ctx, c, entry, "missing_padding_length")
		return
	}
	paddingLen := binary.BigEndian.Uint16(by)
	if paddingLen > 0 {
		_, err = b.ReadBytes(int(paddingLen))
		if err != nil {
			b.Resize(0, n)
			fallback(ctx, c, entry, "invalid_padding")
			return
		}
	}

	session := session.NewServerSession(c, func(stream *session.Stream) {
		defer func() {
			if r := recover(); r != nil {
				entry.WithFields(logrus.Fields{"event": "panic", "stream_id": stream.StreamID(), "panic": r}).Errorln(string(debug.Stack()))
			}
		}()
		defer stream.Close()

		destination, err := M.SocksaddrSerializer.ReadAddrPort(stream)
		if err != nil {
			entry.WithFields(logrus.Fields{"event": "read_target_failed", "stream_id": stream.StreamID()}).WithError(err).Debug("failed to read target address")
			return
		}

		requestFields := cloneLogFields(entry.Data)
		requestFields["stream_id"] = stream.StreamID()
		requestFields["target"] = destination.String()
		eventLogger("inbound", requestFields, "access_target").Debug(formatAccessTargetMessageWithTag(connTag, userID, destination.String()))

		if strings.Contains(destination.String(), "udp-over-tcp.arpa") {
			proxyOutboundUoT(ctx, stream, destination, requestFields)
		} else {
			proxyOutboundTCP(ctx, stream, destination, requestFields)
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
	entry.WithField("event", "connection_end").Debug(formatConnectionLifecycleMessage(connTag, remoteAddr, "end"))
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

func fallback(ctx context.Context, c net.Conn, entry *logrus.Entry, reason string) {
	// 暂未实现
	fields := logrus.Fields{"reason": reason}
	if c != nil && c.RemoteAddr() != nil {
		fields["remote_addr"] = fmt.Sprint(c.RemoteAddr())
	}
	if entry != nil {
		entry.WithFields(fields).WithField("event", "fallback").Debug("fallback handler invoked")
		return
	}
	eventLogger("inbound", fields, "fallback").Debug("fallback handler invoked")
}
