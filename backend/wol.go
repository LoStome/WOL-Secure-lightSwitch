//creates a wol packet and sends it to the passed address

package main

import (
	"fmt"
	"net"
	"strings"
)


func getBroadcastAddr(skipList []string) (string, error) {

    
	//calculates the broadcast address for the current network (mask, interface, etc) and returns it as a string with the port 9 (the default WoL port)
    interfaces, _ := net.Interfaces()
    for _, iface := range interfaces {

		fmt.Printf("Checking interface: %s\n", iface.Name)
        
		//checks if the interface should be skipped based on the environment variable SKIP_INTERFACES
        shouldSkip := false
        for _, skipName := range skipList {
            if skipName != "" && strings.Contains(strings.ToLower(iface.Name), strings.ToLower(skipName)) {
                shouldSkip = true
                break
            }
        }

        if shouldSkip || iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
            continue
        }
        addrs, _ := iface.Addrs()

		fmt.Printf("Interface: %s, Addrs: %v\n", iface.Name, addrs)
        for _, addr := range addrs {
            ipNet, ok := addr.(*net.IPNet)
            if ok && ipNet.IP.To4() != nil {
                
                ip := ipNet.IP.To4()

				//skips auto config addresses (169.254.x.x)
        		if ip[0] == 169 && ip[1] == 254 {
            	continue
        		}

				//calculates the broadcast address
                mask := ipNet.Mask
				fmt.Printf("IP: %s, Mask: %s\n", ip, mask)
                broadcast := net.IP(make([]byte, 4))
                for i := 0; i < 4; i++ {
                    broadcast[i] = ip[i] | ^mask[i]
                }
                return broadcast.String() + ":9", nil
            }
        }
    }
    return "", fmt.Errorf("no interface with valid IPv4 address found")
}

func SendWol(h *Host) error{
    macString := h.MAC

	fmt.Printf("Sending WoL packet to %s...\n", macString)

	hwAddr, err := net.ParseMAC(macString);
    if err != nil {
        return err	
	}

	// create packet
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {        
    packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
        // Copy the hardware address into the packet 16 times
        copy(packet[6+(i*6):], hwAddr)
    }
	//

	//send packet
	broadcastStr, err := getBroadcastAddr(h.SkipInterfaces)
    if err != nil {
        return err
    }

    addr, err := net.ResolveUDPAddr("udp", broadcastStr)
    if err != nil {
        return err
    }

    conn, err := net.DialUDP("udp", nil, addr)
    if err != nil {
        return err
    }
    defer conn.Close()

    _, err = conn.Write(packet)
    if err != nil {
        return err
    }
	//

	fmt.Printf("Magic Packet sent to %s\n", macString)
    return nil
}
