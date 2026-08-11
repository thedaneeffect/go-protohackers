package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"unsafe"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	addr := flag.String("addr", ":12345", "listen address")
	flag.Parse()
	listen_addr, err := net.ResolveUDPAddr("udp", *addr)
	if err != nil {
		panic(fmt.Errorf("bad udp addr: %w", err))
	}
	listener, err := net.ListenUDP("udp", listen_addr)
	if err != nil {
		panic(fmt.Errorf("listen: %w", err))
	}
	log.Printf("Listening on: %+v...", *addr)
	buf := make([]byte, 1000)
	var db sync.Map
	db.Store("version", "Dane's Terrible KV Store v1.0")
	for {
		n, remote_addr, err := listener.ReadFromUDP(buf[:])
		req := string(buf[:n])
		log.Printf("%s: req: %q", remote_addr, req)
		if err != nil {
			log.Println(fmt.Errorf("%s: read: %w", remote_addr, err))
			continue
		}
		key, value, insert := strings.Cut(req, "=")
		if insert {
			if key == "version" { // version is read-only
				continue
			}
			db.Store(key, value)
		} else {
			if loaded, ok := db.Load(key); ok {
				value = loaded.(string)
			} else {
				value = ""
			}
			// it shouldn't be possible to make this overflow since it can only
			// ever respond with what was set, and the incoming message is
			// limited to 1000 bytes.
			n, err = fmt.Fprintf(bytes.NewBuffer(buf[:0]), "%s=%s", key, value)
			if err != nil {
				log.Print(fmt.Errorf("%s: bad Fprintf: (key=%s, value=%s): %w", remote_addr, key, value, err))
				continue
			}
			// unsafe just to micro-optimize
			log.Printf("%s: resp: %q", remote_addr, unsafe.String(&buf[0], n))
			if _, err = listener.WriteToUDP(buf[:n], remote_addr); err != nil {
				log.Print(fmt.Errorf("%s: bad write: %w", remote_addr, err))
				continue
			}
		}
	}
}
