package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"unicode"
)

var connected sync.Map // [conn:net.Conn] name:string

func broadcast(skipped net.Conn, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	connected.Range(func(key, value any) bool {
		if conn := key.(net.Conn); conn != skipped {
			_, _ = fmt.Fprintln(conn, message)
		}
		return true
	})
}

func connectedNames() []string {
	var names []string
	connected.Range(func(key, value any) bool {
		names = append(names, value.(string))
		return true
	})
	return names
}

func main() {
	listen_addr := flag.String("addr", ":12345", "listen address")
	flag.Parse()

	listener, err := net.Listen("tcp", *listen_addr)

	if err != nil {
		panic(fmt.Errorf("listen: %w", err))
	}

	log.Printf("Listening on: %+v...\n", *listen_addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(fmt.Errorf("accept: %w", err))
		}

		log.Printf("accept: %s\n", conn.RemoteAddr())

		go handle(conn)
	}
}

func handle(conn net.Conn) {
	addr := conn.RemoteAddr().String()

	defer log.Printf("close: %s\n", addr)
	defer conn.Close()

	if _, err := fmt.Fprintln(conn, "Welcome to budgetchat! What shall I call you?"); err != nil {
		log.Printf("%s\n", fmt.Errorf("%s: first message failed: %w", addr, err))
		return
	}

	name, err := readName(conn)

	if err != nil {
		log.Printf("%s\n", fmt.Errorf("%s: bad name: %w", addr, err))
		return
	}

	log.Printf("%s: accepted name: %q", addr, name)

	other_names := connectedNames()

	if len(other_names) == 0 {
		if _, err := fmt.Fprintln(conn, "* The room is empty"); err != nil {
			return
		}
	} else {
		joined_names := strings.Join(other_names, ", ")
		if _, err := fmt.Fprintf(conn, "* The room contains: %s\n", joined_names); err != nil {
			return
		}
	}

	connected.Store(conn, name)
	broadcast(conn, "* %s has entered the room", name)

	defer func(conn net.Conn) {
		connected.Delete(conn)
		broadcast(nil, "* %s has left the room", name)
	}(conn)

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		broadcast(conn, "[%s] %s", name, scanner.Text())
	}

	if err = scanner.Err(); err != nil {
		log.Printf("%s\n", fmt.Errorf("%s: scanner: %w", addr, err))
	}
}

func readName(conn net.Conn) (string, error) {
	var buf [64]byte
	n, err := io.ReadAtLeast(conn, buf[:], 1)
	if err != nil {
		return "", err
	}
	name := string(buf[:n-1]) // - 1 to ignore newline
	for _, r := range name {
		if unicode.IsDigit(r) || unicode.IsLower(r) || unicode.IsUpper(r) {
			continue
		}
		return "", fmt.Errorf("invalid character in name: %c", r)
	}
	return name, nil
}
