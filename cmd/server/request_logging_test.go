package main

import (
	"net"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestCloneLogFields(t *testing.T) {
	original := logrus.Fields{
		"user_id":   int64(1),
		"remote_ip": "1.2.3.4",
	}

	cloned := cloneLogFields(original)
	cloned["remote_ip"] = "5.6.7.8"
	cloned["target"] = "example.com:443"

	if got := original["remote_ip"]; got != "1.2.3.4" {
		t.Fatalf("original remote_ip = %v, want %q", got, "1.2.3.4")
	}
	if _, ok := original["target"]; ok {
		t.Fatal("original fields unexpectedly mutated")
	}
}

func TestAddrString(t *testing.T) {
	if got := addrString(nil); got != "" {
		t.Fatalf("addrString(nil) = %q, want empty string", got)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 443}
	if got := addrString(addr); got != "127.0.0.1:443" {
		t.Fatalf("addrString() = %q, want %q", got, "127.0.0.1:443")
	}
}

func TestFormatAccessTargetMessage(t *testing.T) {
	if got := formatAccessTargetMessage(1, "www.google.com:443"); got != "user 1 access address: www.google.com:443" {
		t.Fatalf("formatAccessTargetMessage() = %q", got)
	}
	if got := formatAccessTargetMessage(0, "www.google.com:443"); got != "access address: www.google.com:443" {
		t.Fatalf("formatAccessTargetMessage() anonymous = %q", got)
	}
	if got := formatAccessTargetMessageWithTag("{1:49208}", 1, "www.google.com:443"); got != "{1:49208} user 1 access address: www.google.com:443" {
		t.Fatalf("formatAccessTargetMessageWithTag() = %q", got)
	}
}

func TestFormatOutboundConnectedMessage(t *testing.T) {
	if got := formatOutboundConnectedMessage("tcp", "172.26.0.2:12345", "www.google.com:443", "142.251.1.1:443"); got != "tcp direct dial 172.26.0.2:12345 -> www.google.com:443 (142.251.1.1:443)" {
		t.Fatalf("formatOutboundConnectedMessage() tcp = %q", got)
	}
	if got := formatOutboundConnectedMessage("udp-over-tcp", "0.0.0.0:53531", "8.8.8.8:53", ""); got != "udp-over-tcp direct dial 0.0.0.0:53531 -> 8.8.8.8:53" {
		t.Fatalf("formatOutboundConnectedMessage() uot = %q", got)
	}
	if got := formatOutboundConnectedMessageWithTag("{1:49208}", "tcp", "172.26.0.2:12345", "www.google.com:443", "142.251.1.1:443"); got != "{1:49208} tcp direct dial 172.26.0.2:12345 -> www.google.com:443 (142.251.1.1:443)" {
		t.Fatalf("formatOutboundConnectedMessageWithTag() tcp = %q", got)
	}
}

func TestBuildConnectionTag(t *testing.T) {
	if got := buildConnectionTag(1, "39.64.247.198:49208"); got != "{1:49208}" {
		t.Fatalf("buildConnectionTag() = %q", got)
	}
	if got := buildConnectionTag(0, "39.64.247.198:49208"); got != "{?:49208}" {
		t.Fatalf("buildConnectionTag() anonymous = %q", got)
	}
}

func TestFormatConnectionLifecycleMessage(t *testing.T) {
	if got := formatConnectionLifecycleMessage("{1:49208}", "39.64.247.198:49208", "start"); got != "{1:49208} start handle client 39.64.247.198:49208 connection" {
		t.Fatalf("formatConnectionLifecycleMessage(start) = %q", got)
	}
	if got := formatConnectionLifecycleMessage("{1:49208}", "39.64.247.198:49208", "end"); got != "{1:49208} handle anytls end" {
		t.Fatalf("formatConnectionLifecycleMessage(end) = %q", got)
	}
}

func TestFormatDomainResolvedMessage(t *testing.T) {
	if got := formatDomainResolvedMessage("{1:49208}", "bing.com", []string{"150.171.28.10", "2620:1ec:33:1::10"}); got != "{1:49208} domain bing.com resolved: [150.171.28.10 2620:1ec:33:1::10]" {
		t.Fatalf("formatDomainResolvedMessage() = %q", got)
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "ipv4", input: "39.64.247.198:49208", expect: "49208"},
		{name: "ipv6", input: "[2404:6800:400a:1002::63]:443", expect: "443"},
		{name: "empty", input: "", expect: ""},
		{name: "invalid", input: "invalid-address", expect: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePort(tt.input); got != tt.expect {
				t.Fatalf("parsePort(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestConnTagFromFields(t *testing.T) {
	if got := connTagFromFields(nil); got != "" {
		t.Fatalf("connTagFromFields(nil) = %q, want empty", got)
	}
	if got := connTagFromFields(logrus.Fields{"conn_tag": nil}); got != "" {
		t.Fatalf("connTagFromFields(nil value) = %q, want empty", got)
	}
	if got := connTagFromFields(logrus.Fields{"conn_tag": "{1:49208}"}); got != "{1:49208}" {
		t.Fatalf("connTagFromFields(valid) = %q", got)
	}
}
