package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
)

type data struct {
	timestamp int32
	price     int32
}

var zero [4]byte

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
			addr := c.RemoteAddr().String()

			defer fmt.Printf("close: %s\n", c.RemoteAddr())
			defer conn.Close()

			var records []data

		handle_message:
			for {
				var message [9]byte

				if _, err := io.ReadFull(c, message[:]); err != nil {
					if err == io.EOF {
						return
					}
					fmt.Printf("%s\n", fmt.Errorf("%s: bad message: %w", addr, err))
					continue handle_message
				}

				fmt.Printf("%s: read: %+v\n", addr, message)

				switch typ := message[0]; typ {
				case 'I':
					records = append(records, data{
						timestamp: int32(binary.BigEndian.Uint32(message[1:])),
						price:     int32(binary.BigEndian.Uint32(message[5:])),
					})
				case 'Q':
					mintime := int32(binary.BigEndian.Uint32(message[1:]))
					maxtime := int32(binary.BigEndian.Uint32(message[5:]))

					if mintime > maxtime {
						c.Write(zero[:])
						continue handle_message
					}

					var sum int64
					var n int64

					for _, record := range records {
						if record.timestamp >= mintime && record.timestamp <= maxtime {
							sum += int64(record.price)
							n++
						}
					}

					var buf [4]byte

					if n != 0 {
						mean := int32(sum / n)
						binary.BigEndian.PutUint32(buf[:], uint32(mean))
					}

					if _, err := c.Write(buf[:]); err != nil {
						fmt.Printf("%s\n", fmt.Errorf("%s: bad write: %w", addr, err))
						continue handle_message
					}
				default:
					fmt.Printf("%s\n", fmt.Errorf("%s: unknown message type: %q", addr, string(typ)))
					continue handle_message
				}
			}

		}(conn)
	}
}
