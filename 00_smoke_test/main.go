package main

import (
	"flag"
	"fmt"
	"io"
	"net"
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
	addr := flag.String("addr", ":12345", "listen address")
	flag.Parse()

	listener, err := net.Listen("tcp", *addr)

	if err != nil {
		panic(fmt.Errorf("listen: %w", err))
	}

	fmt.Printf("Listening on: %+v...\n", *addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(fmt.Errorf("accept: %w", err))
		}

		fmt.Printf("accept: %s\n", conn.RemoteAddr())

		go func(c net.Conn) {
			defer fmt.Printf("close: %s\n", c.RemoteAddr())
			defer conn.Close()
			// pipe it straight back
			if _, err := io.Copy(c, &dbgr{Addr: c.RemoteAddr(), Reader: c}); err != nil {
				panic(fmt.Errorf("write: %s: %w", c.RemoteAddr(), err))
			}
		}(conn)
	}
}
