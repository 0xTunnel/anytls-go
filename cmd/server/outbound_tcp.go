package main

import (
	"anytls/proxy"
	"context"
	"net"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
	"github.com/sirupsen/logrus"
)

func proxyOutboundTCP(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	c, err := proxy.SystemDialer.DialContext(ctx, "tcp", destination.String())
	if err != nil {
		eventLogger("outbound", logrus.Fields{"target": destination.String(), "transport": "tcp"}, "dial_failed").WithError(err).Warn("outbound dial failed")
		err = E.Errors(err, N.ReportHandshakeFailure(conn, err))
		return err
	}
	defer c.Close()

	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		return err
	}

	return bufio.CopyConn(ctx, conn, c)
}

func proxyOutboundUoT(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	request, err := uot.ReadRequest(conn)
	if err != nil {
		eventLogger("outbound", logrus.Fields{"target": destination.String(), "transport": "udp-over-tcp"}, "read_request_failed").WithError(err).Warn("read udp-over-tcp request failed")
		return err
	}

	c, err := net.ListenPacket("udp", "")
	if err != nil {
		eventLogger("outbound", logrus.Fields{"target": destination.String(), "transport": "udp-over-tcp"}, "listen_packet_failed").WithError(err).Error("create udp packet listener failed")
		err = E.Errors(err, N.ReportHandshakeFailure(conn, err))
		return err
	}
	defer c.Close()

	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		return err
	}

	return bufio.CopyPacketConn(ctx, uot.NewConn(conn, *request), bufio.NewPacketConn(c))
}
