package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func cloneLogFields(fields logrus.Fields) logrus.Fields {
	if len(fields) == 0 {
		return logrus.Fields{}
	}
	cloned := make(logrus.Fields, len(fields))
	for key, value := range fields {
		cloned[key] = value
	}
	return cloned
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

func formatAccessTargetMessage(userID int64, target string) string {
	return formatAccessTargetMessageWithTag("", userID, target)
}

func formatAccessTargetMessageWithTag(connTag string, userID int64, target string) string {
	if userID > 0 {
		return withConnTagPrefix(connTag, fmt.Sprintf("user %d access address: %s", userID, target))
	}
	return withConnTagPrefix(connTag, fmt.Sprintf("access address: %s", target))
}

func formatOutboundConnectedMessage(transport string, localAddr string, target string, outboundAddr string) string {
	return formatOutboundConnectedMessageWithTag("", transport, localAddr, target, outboundAddr)
}

func formatOutboundConnectedMessageWithTag(connTag string, transport string, localAddr string, target string, outboundAddr string) string {
	if outboundAddr != "" {
		return withConnTagPrefix(connTag, fmt.Sprintf("%s direct dial %s -> %s (%s)", transport, localAddr, target, outboundAddr))
	}
	if localAddr != "" {
		return withConnTagPrefix(connTag, fmt.Sprintf("%s direct dial %s -> %s", transport, localAddr, target))
	}
	return withConnTagPrefix(connTag, fmt.Sprintf("%s direct dial -> %s", transport, target))
}

func formatConnectionLifecycleMessage(connTag string, remoteAddr string, phase string) string {
	switch phase {
	case "start":
		return withConnTagPrefix(connTag, fmt.Sprintf("start handle client %s connection", remoteAddr))
	case "end":
		return withConnTagPrefix(connTag, "handle anytls end")
	default:
		return withConnTagPrefix(connTag, phase)
	}
}

func formatDomainResolvedMessage(connTag string, domain string, addresses []string) string {
	return withConnTagPrefix(connTag, fmt.Sprintf("domain %s resolved: [%s]", domain, strings.Join(addresses, " ")))
}

func buildConnectionTag(userID int64, remoteAddr string) string {
	userPart := "?"
	if userID > 0 {
		userPart = strconv.FormatInt(userID, 10)
	}
	port := parsePort(remoteAddr)
	if port == "" {
		port = "?"
	}
	return fmt.Sprintf("{%s:%s}", userPart, port)
}

func parsePort(remoteAddr string) string {
	if strings.TrimSpace(remoteAddr) == "" {
		return ""
	}
	_, port, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return port
	}
	last := strings.LastIndex(remoteAddr, ":")
	if last < 0 || last == len(remoteAddr)-1 {
		return ""
	}
	return remoteAddr[last+1:]
}

func withConnTagPrefix(connTag string, message string) string {
	if connTag == "" {
		return message
	}
	return connTag + " " + message
}

func connTagFromFields(fields logrus.Fields) string {
	if fields == nil {
		return ""
	}
	raw, ok := fields["conn_tag"]
	if !ok || raw == nil {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
