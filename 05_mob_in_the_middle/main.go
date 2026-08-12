package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"unicode"
)

func main() {
	log.SetFlags(log.Lshortfile)
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

	proxy, err := net.Dial("tcp", "chat.protohackers.com:16963")
	if err != nil {
		log.Println(fmt.Errorf("%s: connect: %w", addr, err))
		return
	}
	defer proxy.Close()

	go func() {
		defer conn.Close() // unblocks scanner.Scan() below
		proxy_scanner := bufio.NewScanner(proxy)
		for proxy_scanner.Scan() {
			text := replaceBoguscoin(proxy_scanner.Text())
			log.Printf("%s <- %q", addr, text)
			if _, err := conn.Write([]byte(text + "\n")); err != nil {
				log.Println(fmt.Errorf("%s: <-proxy: %w", addr, err))
				break
			}
		}
		if err := proxy_scanner.Err(); err != nil {
			log.Println(fmt.Errorf("%s: proxy scanner: %w", addr, err))
		}
	}()

	// we use a reader here instead of a scanner because can't allow the client
	// to send partial messages that don't end with a newline. It's possible
	// for the client to have sent `hello` without `\n` and the scanner to only
	// pick it up after the disconnect
	client_reader := bufio.NewReader(conn)
	for {
		text, err := client_reader.ReadString('\n')
		if err != nil {
			log.Println(fmt.Errorf("%s: client reader: %w", addr, err))
			break
		}
		text = replaceBoguscoin(text[:len(text)-1]) // trim \n
		log.Printf("%s -> %q", addr, text)
		if _, err := proxy.Write([]byte(text + "\n")); err != nil {
			log.Println(fmt.Errorf("%s: proxy<-: %w", addr, err))
			break
		}
	}
}

func replaceBoguscoin(input string) (result string) {
	parts := strings.Split(input, " ")
	found := false
	for i, part := range parts {
		if isBoguscoin(part) {
			parts[i] = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"
			found = true
		}
	}
	if found {
		return strings.Join(parts, " ")
	}
	return input
}

func isBoguscoin(input string) bool {
	l := len(input)
	if l >= 26 && l <= 35 && input[0] == '7' {
		valid := true
		for _, r := range input[1:] {
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
				valid = false
				break
			}
		}
		return valid
	}
	return false
}
