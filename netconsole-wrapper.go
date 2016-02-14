// Copyright (c) 2016 Dominik Zeromski <dzeromsk@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
)

var (
	dstaddr = flag.String("server", "netconsole.srv:6666", "netconsole server address")

	// netconsole=[+][src-port]@[src-ip]/[<dev>],[tgt-port]@<tgt-ip>/[tgt-macaddr]
	param = "netconsole=@%s/%s,%s@%s/%s"
)

func InterfaceByIP(x net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if x.Equal(ip.To4()) {
				return &iface, nil
			}
		}
	}
	return nil, errors.New("no such network interface")
}

func HardwareAddrByIP(x net.IP) (*net.HardwareAddr, error) {
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if x.Equal(net.ParseIP(fields[0])) {
			addr, err := net.ParseMAC(fields[3])
			if err != nil {
				continue
			}

			return &addr, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, errors.New("no mac address found")
}

func GatewayIP() (x *net.IP, err error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if fields[7] != "00000000" {
			continue
		}
		b, err := hex.DecodeString(fields[2])
		if err != nil {
			continue
		}
		ip := net.IPv4(b[3], b[2], b[1], b[0])
		return &ip, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, errors.New("no gw found")
}

func main() {
	flag.Parse()
	addr, err := net.ResolveUDPAddr("udp", *dstaddr)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	conn.Write([]byte("netconsole-wrapper started\n"))

	lip, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		log.Fatal(err)
	}

	rip, rport, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		log.Fatal(err)
	}

	iface, err := InterfaceByIP(net.ParseIP(lip))
	if err != nil {
		log.Fatal(err)
	}

	mac, err := HardwareAddrByIP(net.ParseIP(rip))
	if err != nil {
		gwip, err := GatewayIP()
		if err != nil {
			log.Fatal(err)
		}
		mac, err = HardwareAddrByIP(*gwip)
		if err != nil {
			log.Fatal(err)
		}
	}

	param := fmt.Sprintf(param, lip, iface.Name, rport, rip, mac)

	modprobe := exec.Command("modprobe", "netconsole", param)

	modprobe.Stdout = os.Stdout
	modprobe.Stderr = os.Stderr

	if err := modprobe.Run(); err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
