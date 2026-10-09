package netguard

import (
	"net/netip"
	"testing"
)

func TestPublicIP(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "240.0.0.1", "::1", "fc00::1", "fe80::1", "::ffff:127.0.0.1", "2001:db8::1", "64:ff9b::7f00:1"} {
		if PublicIP(netip.MustParseAddr(value)) {
			t.Fatalf("non-public address accepted: %s", value)
		}
	}
	for _, value := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !PublicIP(netip.MustParseAddr(value)) {
			t.Fatal("public address rejected")
		}
	}
}
func FuzzPublicIP(f *testing.F) {
	f.Add("127.0.0.1")
	f.Add("::ffff:10.0.0.1")
	f.Fuzz(func(t *testing.T, value string) {
		ip, err := netip.ParseAddr(value)
		if err != nil {
			return
		}
		if PublicIP(ip) && (ip.Unmap().IsPrivate() || ip.Unmap().IsLoopback()) {
			t.Fatal("private address accepted")
		}
	})
}
