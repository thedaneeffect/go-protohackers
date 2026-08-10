package main

import (
	"fmt"
	"io"
	"net"
	"net/netip"
)

type dbgr struct {
	net.Addr
	io.Reader
}

func (r *dbgr) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err == nil {
		fmt.Printf("read: %s: [%d]%+v\n", r.Addr, n, p[:n])
	}
	return n, err
}

func main() {
	addr := net.TCPAddrFromAddrPort(netip.MustParseAddrPort("0.0.0.0:12345"))
	fmt.Printf("Listening on: %+v...\n", addr)
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(fmt.Errorf("listen: %w", err))
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			panic(fmt.Errorf("accept: %w", err))
		}

		fmt.Printf("accept: %s\n", conn.RemoteAddr())

		go func(c *net.TCPConn) {
			defer fmt.Printf("close: %s\n", c.RemoteAddr())
			defer conn.Close()
			// pipe it straight back
			if _, err := io.Copy(c, &dbgr{Addr: c.RemoteAddr(), Reader: c}); err != nil {
				panic(fmt.Errorf("write: %s: %w", c.RemoteAddr(), err))
			}
		}(conn)
	}
}
