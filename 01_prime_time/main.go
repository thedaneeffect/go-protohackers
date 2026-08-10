package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/netip"
)

type request struct {
	Method string   `json:"method"`
	Number *float64 `json:"number"`
}

type response struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
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

		go func(c *net.TCPConn) {
			addr := conn.RemoteAddr()

			fmt.Printf("open: %s\n", addr)
			defer fmt.Printf("close: %s\n", addr)

			defer conn.Close()

			reader := bufio.NewReader(c)
			writer := bufio.NewWriter(c)

			for {
				var req request
				req_data, err := reader.ReadBytes('\n')

				if err == io.EOF {
					break
				} else if err != nil {
					fmt.Printf("%s\n", fmt.Errorf("%s: bad read: %w", addr, err))
					return
				} else if err := json.Unmarshal(req_data, &req); err != nil {
					fmt.Printf("%s\n", fmt.Errorf("%s: bad unmarshal: %w", addr, err))

					_, _ = writer.WriteString(err.Error())
					_ = writer.Flush()
					return
				}

				if req.Method != "isPrime" {
					_, _ = fmt.Fprintf(writer, "invalid method: %q", req.Method)
					_ = writer.Flush()
					return
				}

				if req.Number == nil {
					_, _ = fmt.Fprint(writer, "missing number")
					_ = writer.Flush()
					return
				}

				fmt.Printf("%s: request: %+v\n", addr, req)
				res := response{
					Method: req.Method,
					Prime:  big.NewInt(int64(*req.Number)).ProbablyPrime(0),
				}

				res_data, err := json.Marshal(res)
				if err != nil {
					fmt.Printf("%s\n", fmt.Errorf("%s: bad marshal: %w", addr, err))
					return
				}

				fmt.Printf("%s: response: %+v\n", addr, string(res_data))

				_, _ = writer.Write(res_data)
				_ = writer.WriteByte('\n')

				if err := writer.Flush(); err != nil {
					fmt.Printf("%s\n", fmt.Errorf("%s: bad flush: %w", addr, err))
					return
				}

			}
		}(conn)
	}
}
