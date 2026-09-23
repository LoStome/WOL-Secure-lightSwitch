package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func fakeWolNetwork(t *testing.T, addresses ...string) wolNetwork {
	t.Helper()
	return wolNetwork{
		interfaces: func() ([]net.Interface, error) {
			var result []net.Interface
			for i := range addresses {
				result = append(result, net.Interface{Index: i, Name: []string{"other", "lan", "extra"}[i], Flags: net.FlagUp | net.FlagBroadcast})
			}
			return result, nil
		},
		addrs: func(i net.Interface) ([]net.Addr, error) {
			ip, subnet, err := net.ParseCIDR(addresses[i.Index])
			if err != nil {
				t.Fatal(err)
			}
			subnet.IP = ip
			return []net.Addr{subnet}, nil
		},
		send: func(local, remote *net.UDPAddr, packet []byte) error { t.Fatal("unexpected network send"); return nil },
	}
}

func TestWolSelectsDeviceNetwork(t *testing.T) {
	network := fakeWolNetwork(t, "10.0.0.2/24", "192.168.1.2/24")
	sent := false
	network.send = func(local, remote *net.UDPAddr, packet []byte) error {
		sent = true
		if remote.String() != "192.168.1.255:9" {
			t.Errorf("destination = %s, want 192.168.1.255:9", remote)
		}
		if local == nil || !local.IP.Equal(net.ParseIP("192.168.1.2")) {
			t.Errorf("source = %v, want 192.168.1.2", local)
		}
		return nil
	}
	if err := sendWol(&Host{MAC: "00:11:22:33:44:55", IP: "192.168.1.100"}, network); err != nil {
		t.Fatal(err)
	}
	if !sent {
		t.Fatal("packet not sent")
	}
}

func TestWolSelectionAndPacket(t *testing.T) {
	cases := []struct {
		name      string
		addresses []string
		host      Host
		want      string
	}{
		{"single without IP", []string{"10.0.0.2/24"}, Host{}, "10.0.0.255:9"},
		{"ambiguous without IP", []string{"10.0.0.2/24", "192.168.1.2/24"}, Host{}, ""},
		{"explicit without IP", []string{"10.0.0.2/24", "192.168.1.2/24"}, Host{WolInterface: "lan"}, "192.168.1.255:9"},
		{"explicit wins", []string{"10.0.0.2/24", "192.168.1.2/24"}, Host{WolInterface: "lan", IP: "10.0.0.100"}, "192.168.1.255:9"},
		{"explicit skips DNS", []string{"10.0.0.2/24"}, Host{WolInterface: "other", IP: "invalid.example"}, "10.0.0.255:9"},
		{"skip without IP", []string{"10.0.0.2/24", "192.168.1.2/24"}, Host{SkipInterfaces: []string{"OTH"}}, "192.168.1.255:9"},
		{"explicit excluded", []string{"10.0.0.2/24"}, Host{WolInterface: "other", SkipInterfaces: []string{"OTHER"}}, ""},
		{"exact name", []string{"10.0.0.2/24"}, Host{WolInterface: "Other"}, ""},
		{"no matching subnet", []string{"10.0.0.2/24"}, Host{IP: "192.168.1.100"}, ""},
		{"longest prefix", []string{"10.0.0.2/16", "10.0.1.2/24"}, Host{IP: "10.0.1.100"}, "10.0.1.255:9"},
		{"equal prefix ambiguous", []string{"10.0.0.2/24", "10.0.0.3/24"}, Host{IP: "10.0.0.100"}, ""},
		{"IPv6 target", []string{"10.0.0.2/24"}, Host{IP: "2001:db8::1"}, ""},
		{"link local excluded", []string{"169.254.1.2/16"}, Host{}, ""},
		{"IPv6 interface", []string{"2001:db8::2/64"}, Host{}, ""},
		{"no broadcast subnet", []string{"10.0.0.2/31"}, Host{}, ""},
		{"no interfaces", nil, Host{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			network := fakeWolNetwork(t, tc.addresses...)
			sent := false
			network.send = func(local, remote *net.UDPAddr, packet []byte) error {
				sent = true
				if remote.String() != tc.want {
					t.Errorf("destination = %s, want %s", remote, tc.want)
				}
				if local == nil || local.IP.To4() == nil || local.Port != 0 {
					t.Errorf("invalid local address: %v", local)
				}
				expected := append(bytes.Repeat([]byte{0xff}, 6), bytes.Repeat([]byte{0, 17, 34, 51, 68, 85}, 16)...)
				if !bytes.Equal(packet, expected) {
					t.Errorf("invalid magic packet: %x", packet)
				}
				return nil
			}
			tc.host.MAC = "00:11:22:33:44:55"
			err := sendWol(&tc.host, network)
			if tc.want == "" {
				if err == nil || sent {
					t.Fatalf("expected error without sending, got %v, sent %v", err, sent)
				}
			} else if err != nil || !sent {
				t.Fatalf("err=%v sent=%v", err, sent)
			}
		})
	}
}

func TestWolErrors(t *testing.T) {
	sentinel := errors.New("fake network error")
	for _, operation := range []string{"interfaces", "addresses", "lookup", "send"} {
		t.Run(operation, func(t *testing.T) {
			network := fakeWolNetwork(t, "10.0.0.2/24")
			host := Host{MAC: "00:11:22:33:44:55", IP: "10.0.0.100"}
			switch operation {
			case "interfaces":
				network.interfaces = func() ([]net.Interface, error) { return nil, sentinel }
			case "addresses":
				network.addrs = func(net.Interface) ([]net.Addr, error) { return nil, sentinel }
			case "lookup":
				host.IP = "device.example"
				network.lookup = func(ctx context.Context, name string) ([]net.IP, error) {
					if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 2*time.Second {
						t.Error("missing bounded DNS timeout")
					}
					return nil, sentinel
				}
			case "send":
				network.send = func(*net.UDPAddr, *net.UDPAddr, []byte) error { return sentinel }
			}
			if err := sendWol(&host, network); !errors.Is(err, sentinel) {
				t.Fatalf("lost error: %v", err)
			}
		})
	}
	for _, mac := range []string{"invalid", "00:11:22:33:44:55:66:77"} {
		if err := sendWol(&Host{MAC: mac}, wolNetwork{}); err == nil {
			t.Errorf("accepted MAC %s", mac)
		}
	}
}

func TestWolHostnameAndMultipleSubnets(t *testing.T) {
	for _, target := range []string{"", "10.0.1.100", "device.example", "unmatched.example"} {
		t.Run(target, func(t *testing.T) {
			network := fakeWolNetwork(t, "10.0.0.2/24")
			network.addrs = func(net.Interface) ([]net.Addr, error) {
				return []net.Addr{&net.IPNet{IP: net.ParseIP("10.0.0.2"), Mask: net.CIDRMask(24, 32)}, &net.IPNet{IP: net.ParseIP("10.0.1.2"), Mask: net.CIDRMask(24, 32)}}, nil
			}
			network.lookup = func(ctx context.Context, name string) ([]net.IP, error) {
				if name == "unmatched.example" {
					return []net.IP{net.ParseIP("192.168.1.1")}, nil
				}
				return []net.IP{net.ParseIP("10.0.1.100"), net.ParseIP("10.0.1.101")}, nil
			}
			local, remote, err := selectWolNetwork(&Host{WolInterface: "other", IP: target}, network)
			if target == "" || target == "unmatched.example" {
				if err == nil {
					t.Fatal("expected ambiguous or unmatched network error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if local.IP.String() != "10.0.1.2" || remote.String() != "10.0.1.255:9" {
				t.Fatalf("wrong network: %v %v", local, remote)
			}
		})
	}
}

func TestWolInterfaceFlags(t *testing.T) {
	for _, flags := range []net.Flags{net.FlagBroadcast, net.FlagUp, net.FlagUp | net.FlagBroadcast | net.FlagLoopback} {
		network := fakeWolNetwork(t, "10.0.0.2/24")
		network.interfaces = func() ([]net.Interface, error) { return []net.Interface{{Name: "other", Flags: flags}}, nil }
		if _, _, err := selectWolNetwork(&Host{}, network); err == nil {
			t.Errorf("accepted flags %v", flags)
		}
	}
}

func TestWolInterfaceConfigurationWithoutIP(t *testing.T) {
	hosts, err := parseHosts([]byte("- id: test\n  name: Test\n  mac: '00:11:22:33:44:55'\n  wol_interface: lan\n  skip_interfaces: [other]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].WolInterface != "lan" || hosts[0].IP != "" || len(hosts[0].SkipInterfaces) != 1 {
		t.Fatalf("unexpected hosts: %+v", hosts)
	}
	network := fakeWolNetwork(t, "10.0.0.2/24", "192.168.1.2/24")
	_, remote, err := selectWolNetwork(&hosts[0], network)
	if err != nil {
		t.Fatal(err)
	}
	if remote.String() != "192.168.1.255:9" {
		t.Fatal(remote)
	}
}
