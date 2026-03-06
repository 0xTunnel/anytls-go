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

func proxyOutboundTCP(ctx context.Context, conn net.Conn, destination M.Socksaddr, requestFields logrus.Fields) error {
	connTag := connTagFromFields(requestFields)
	if logrus.IsLevelEnabled(logrus.DebugLevel) && destination.IsFqdn() {
		if ips, err := net.DefaultResolver.LookupIPAddr(ctx, destination.Fqdn); err == nil && len(ips) > 0 {
			resolved := make([]string, 0, len(ips))
			for _, ip := range ips {
				resolved = append(resolved, ip.IP.String())
			}
			eventLogger("outbound", requestFields, "domain_resolved").Debug(formatDomainResolvedMessage(connTag, destination.Fqdn, resolved))
		}
	}
	c, err := proxy.SystemDialer.DialContext(ctx, "tcp", destination.String())
	if err != nil {
		fields := cloneLogFields(requestFields)
		fields["target"] = destination.String()
		fields["transport"] = "tcp"
		eventLogger("outbound", fields, "dial_failed").WithError(err).Warn(withConnTagPrefix(connTag, "tcp direct dial failed"))
		err = E.Errors(err, N.ReportHandshakeFailure(conn, err))
		return err
	}
	defer c.Close()

	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		return err
	}

	fields := cloneLogFields(requestFields)
	fields["target"] = destination.String()
	fields["transport"] = "tcp"
	fields["local_addr"] = addrString(c.LocalAddr())
	fields["outbound_addr"] = addrString(c.RemoteAddr())
	eventLogger("outbound", fields, "outbound_connected").Debug(formatOutboundConnectedMessageWithTag(connTag, "tcp", fields["local_addr"].(string), destination.String(), fields["outbound_addr"].(string)))

	return bufio.CopyConn(ctx, conn, c)
}

func proxyOutboundUoT(ctx context.Context, conn net.Conn, destination M.Socksaddr, requestFields logrus.Fields) error {
	connTag := connTagFromFields(requestFields)
	request, err := uot.ReadRequest(conn)
	if err != nil {
		fields := cloneLogFields(requestFields)
		fields["target"] = destination.String()
		fields["transport"] = "udp-over-tcp"
		eventLogger("outbound", fields, "read_request_failed").WithError(err).Warn(withConnTagPrefix(connTag, "read udp-over-tcp request failed"))
		return err
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) && request.Destination.IsFqdn() {
		if ips, err := net.DefaultResolver.LookupIPAddr(ctx, request.Destination.Fqdn); err == nil && len(ips) > 0 {
			resolved := make([]string, 0, len(ips))
			for _, ip := range ips {
				resolved = append(resolved, ip.IP.String())
			}
			eventLogger("outbound", requestFields, "domain_resolved").Debug(formatDomainResolvedMessage(connTag, request.Destination.Fqdn, resolved))
		}
	}

	c, err := net.ListenPacket("udp", "")
	if err != nil {
		fields := cloneLogFields(requestFields)
		fields["target"] = request.Destination.String()
		fields["transport"] = "udp-over-tcp"
		eventLogger("outbound", fields, "listen_packet_failed").WithError(err).Error(withConnTagPrefix(connTag, "udp-over-tcp direct dial failed"))
		err = E.Errors(err, N.ReportHandshakeFailure(conn, err))
		return err
	}
	defer c.Close()

	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		return err
	}

	fields := cloneLogFields(requestFields)
	fields["target"] = request.Destination.String()
	fields["transport"] = "udp-over-tcp"
	fields["local_addr"] = addrString(c.LocalAddr())
	eventLogger("outbound", fields, "outbound_connected").Debug(formatOutboundConnectedMessageWithTag(connTag, "udp-over-tcp", fields["local_addr"].(string), request.Destination.String(), ""))

	return bufio.CopyPacketConn(ctx, uot.NewConn(conn, *request), bufio.NewPacketConn(c))
}
