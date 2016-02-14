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
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"

	"xi2.org/x/logrot"
)

var (
	logfile  = flag.String("logfile", "netconsoled.log", "netconsoled log file")
	logdir   = flag.String("logdir", ".", "client log file")
	httpaddr = flag.String("http", ":8080", "http server listen address")
	udpaddr  = flag.String("udp", ":6666", "udp server listen address")
)

func main() {
	flag.Parse()

	l, err := logrot.Open(*logfile, 0600, 100<<20, 2)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(l)

	http.Handle("/netconsole/", http.StripPrefix("/netconsole/", http.FileServer(http.Dir(*logdir))))
	go func() { log.Fatal(http.ListenAndServe(*httpaddr, nil)) }()

	addr, err := net.ResolveUDPAddr("udp", *udpaddr)
	if err != nil {
		log.Fatal(err)
	}

	sock, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	sock.SetReadBuffer(1 << 20)

	for {
		data := make([]byte, 1024)
		l, c, err := sock.ReadFromUDP(data)
		if err != nil {
			log.Println(err)
		}
		go func(data []byte, ip net.IP) {
			log.Printf("addr: %s, len: %d", ip, len(data))

			filename := filepath.Join(*logdir, fmt.Sprintf("%s.log", ip))

			w, err := logrot.Open(filename, 0600, 1<<20, 2)
			if err != nil {
				log.Println(err)
			}
			defer w.Close()

			if _, err = w.Write(data); err != nil {
				log.Println(err)
			}

		}(data[0:l], c.IP)
	}
}
