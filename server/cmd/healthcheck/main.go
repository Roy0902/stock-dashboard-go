// Command healthcheck is a tiny container health probe. It talks HTTP over a
// raw socket so the minimal runtime needs neither a shell nor curl, and the
// binary does not pull in the net/http stack.
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	address := net.JoinHostPort("127.0.0.1", port)

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := fmt.Fprintf(conn,
		"GET /healthz HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", address); err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}

	statusLine, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
	fields := strings.Fields(statusLine)
	if len(fields) < 2 || fields[1] != "200" {
		fmt.Fprintln(os.Stderr, "healthcheck: status", strings.TrimSpace(statusLine))
		os.Exit(1)
	}
}
