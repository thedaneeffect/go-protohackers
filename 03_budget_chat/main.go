package main

import (
	"flag"
	"fmt"
	"net"
)

func main() {
	listen_addr := flag.String("addr", ":12345", "listen address")
	flag.Parse()

	listener, err := net.Listen("tcp", *listen_addr)

	if err != nil {
		panic(fmt.Errorf("listen: %w", err))
	}

	fmt.Printf("Listening on: %+v...\n", *listen_addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(fmt.Errorf("accept: %w", err))
		}

		fmt.Printf("accept: %s\n", conn.RemoteAddr())

		go func(c net.Conn) {
			addr := c.RemoteAddr().String()

			defer fmt.Printf("close: %s\n", c.RemoteAddr())
			defer conn.Close()

			// TODO: do stuff
			_ = addr
		}(conn)
	}
}
