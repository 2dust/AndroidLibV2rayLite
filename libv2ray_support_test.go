package libv2ray

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	v2net "github.com/v2fly/v2ray-core/v5/common/net"
)

type fakeProtectSet struct{}

func (f fakeProtectSet) Protect(int) bool {
	return true
}

func TestProtectedDialer_PrepareDomain(t *testing.T) {
	tests := []struct {
		name       string
		domainName string
	}{
		{"Valid Domain", "baidu.com:80"},
	}

	d := NewProtectedDialer(fakeProtectSet{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			go d.PrepareDomain(tt.domainName, done, false)

			time.Sleep(1 * time.Second)
			if d.vServer != nil {
				t.Logf("Current IP: %v", d.vServer.currentIP())
				d.vServer.NextIP()
				t.Logf("Next IP: %v", d.vServer.currentIP())
			} else {
				t.Error("Failed to prepare domain")
			}
		})
	}
}

func TestProtectedDialer_Dial(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"Valid Address", "baidu.com:80", false},
		{"Invalid Address", "invalid.domain:80", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewProtectedDialer(fakeProtectSet{})
			d.currentServer = tt.address

			done := make(chan struct{})
			go d.PrepareDomain(tt.address, done, false)

			time.Sleep(1 * time.Second) // Allow some time for domain preparation

			dest, _ := v2net.ParseDestination("tcp:" + tt.address)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := d.Dial(ctx, nil, dest, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unexpected error: %v, wantErr: %v", err, tt.wantErr)
			}
			if conn != nil {
				_host, _, _ := net.SplitHostPort(tt.address)
				fmt.Fprintf(conn, fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\n\r\n", _host))
				status, err := bufio.NewReader(conn).ReadString('\n')
				t.Logf("Response: %s, Error: %v", status, err)
				conn.Close()
			}
		})
	}
}

func TestResolved_NextIP(t *testing.T) {
	tests := []struct {
		name   string
		IPs    []net.IP
	}{
		{
			"Multiple IPs",
			[]net.IP{
				net.ParseIP("1.2.3.4"),
				net.ParseIP("4.3.2.1"),
				net.ParseIP("1234::1"),
				net.ParseIP("4321::1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &resolved{
				domain: "example.com",
				IPs:    tt.IPs,
			}
			for i := 0; i < len(tt.IPs)*2; i++ { // Test cycling through IPs
				t.Logf("Current IP: %v", r.currentIP())
				r.NextIP()
			}
		})
	}
}
