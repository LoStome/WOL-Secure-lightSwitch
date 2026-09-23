package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Dependencies are passed per call so tests never enumerate or send to real LANs.
type wolNetwork struct {
	interfaces func() ([]net.Interface, error)
	addrs      func(net.Interface) ([]net.Addr, error)
	lookup     func(context.Context, string) ([]net.IP, error)
	send       func(*net.UDPAddr, *net.UDPAddr, []byte) error
}

type wolCandidate struct {
	iface  int
	ip     net.IP
	subnet *net.IPNet
	prefix int
}

func selectWolNetwork(h *Host, network wolNetwork) (*net.UDPAddr, *net.UDPAddr, error) {
	interfaces, err := network.interfaces()
	if err != nil {
		return nil, nil, fmt.Errorf("enumerate WOL interfaces: %w", err)
	}
	var candidates []wolCandidate
	for _, iface := range interfaces {
		if h.WolInterface != "" && iface.Name != h.WolInterface {
			continue
		}
		skipped := false
		for _, name := range h.SkipInterfaces {
			if name != "" && strings.Contains(strings.ToLower(iface.Name), strings.ToLower(name)) {
				skipped = true
				break
			}
		}
		if skipped || iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addresses, err := network.addrs(iface)
		if err != nil {
			return nil, nil, fmt.Errorf("enumerate WOL addresses for %q: %w", iface.Name, err)
		}
		for _, address := range addresses {
			subnet, ok := address.(*net.IPNet)
			if !ok {
				continue
			}
			ip := subnet.IP.To4()
			if ip == nil || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() {
				continue
			}
			prefix, bits := subnet.Mask.Size()
			if bits != 32 || prefix > 30 {
				continue
			} // /31 and /32 have no broadcast host network.
			duplicate := false
			for _, existing := range candidates {
				if existing.iface == iface.Index && existing.ip.Equal(ip) && existing.prefix == prefix {
					duplicate = true
					break
				}
			}
			if !duplicate {
				candidates = append(candidates, wolCandidate{iface.Index, ip, subnet, prefix})
			}
		}
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no eligible WOL IPv4 network; check wol_interface and skip_interfaces")
	}
	// An explicit interface with one address wins even when IP is absent or stale.
	if h.IP != "" && !(h.WolInterface != "" && len(candidates) == 1) {
		ips := []net.IP{net.ParseIP(h.IP)}
		if ips[0] == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			ips, err = network.lookup(ctx, h.IP)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve WOL target: %w", err)
			}
		}
		var matching []wolCandidate
		best := -1
		for _, candidate := range candidates {
			for _, ip := range ips {
				if ip.To4() == nil || !candidate.subnet.Contains(ip) {
					continue
				}
				if candidate.prefix > best {
					matching = nil
					best = candidate.prefix
				}
				if candidate.prefix == best {
					matching = append(matching, candidate)
				}
				break
			}
		}
		candidates = matching
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no local WOL IPv4 network matches the device address")
	}
	if len(candidates) != 1 {
		return nil, nil, fmt.Errorf("ambiguous WOL IPv4 networks; set wol_interface, narrow skip_interfaces or provide an IP to disambiguate interface subnets")
	}
	selected := candidates[0]
	broadcast := make(net.IP, net.IPv4len)
	for i := range broadcast {
		broadcast[i] = selected.ip[i] | ^selected.subnet.Mask[i]
	}
	return &net.UDPAddr{IP: selected.ip}, &net.UDPAddr{IP: broadcast, Port: 9}, nil
}

func SendWol(h *Host) error {
	return sendWol(h, wolNetwork{
		interfaces: net.Interfaces,
		addrs:      func(i net.Interface) ([]net.Addr, error) { return i.Addrs() },
		lookup: func(ctx context.Context, name string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip4", name)
		},
		send: func(local, remote *net.UDPAddr, packet []byte) error {
			conn, err := net.DialUDP("udp4", local, remote)
			if err != nil {
				return err
			}
			defer conn.Close()
			n, err := conn.Write(packet)
			if err == nil && n != len(packet) {
				err = io.ErrShortWrite
			}
			return err
		},
	})
}

func sendWol(h *Host, network wolNetwork) error {
	hwAddr, err := net.ParseMAC(h.MAC)
	if err != nil {
		return fmt.Errorf("parse WOL MAC: %w", err)
	}
	if len(hwAddr) != 6 {
		return fmt.Errorf("WOL requires a six-byte MAC address")
	}
	local, remote, err := selectWolNetwork(h, network)
	if err != nil {
		return err
	}
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:6+(i+1)*6], hwAddr)
	}
	if err := network.send(local, remote, packet); err != nil {
		return fmt.Errorf("send WOL packet: %w", err)
	}
	return nil
}
