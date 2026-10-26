package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/techieKB/disti_cache/internal/client"
	"github.com/techieKB/disti_cache/internal/node"
	"github.com/techieKB/disti_cache/internal/protocol"
	"github.com/techieKB/disti_cache/internal/store"
)

func startServer(t *testing.T, h Handler) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(h)
	go srv.Serve(ln)
	t.Cleanup(srv.Close)
	return ln.Addr().String()
}

func storeHandler() Handler {
	st := store.New(4, 100)
	return HandlerFunc(func(c protocol.Command) string { return node.Execute(st, c) })
}

func TestSetGetOverTCP(t *testing.T) {
	addr := startServer(t, storeHandler())
	if got, err := client.Do(addr, "SET greeting 0 hello world", time.Second); err != nil || got != "OK" {
		t.Fatalf("SET -> %q, %v", got, err)
	}
	if got, _ := client.Do(addr, "GET greeting", time.Second); got != "VALUE hello world" {
		t.Fatalf("GET -> %q", got)
	}
	if got, _ := client.Do(addr, "DEL greeting", time.Second); got != "DELETED" {
		t.Fatalf("DEL -> %q", got)
	}
	if got, _ := client.Do(addr, "GET greeting", time.Second); got != "NOT_FOUND" {
		t.Fatalf("GET after DEL -> %q", got)
	}
}

func TestMalformedCommandGetsErr(t *testing.T) {
	addr := startServer(t, storeHandler())
	got, err := client.Do(addr, "FLY away", time.Second)
	if err != nil || !strings.HasPrefix(got, "ERR ") {
		t.Fatalf("got %q, %v; want ERR ...", got, err)
	}
}

func TestManyCommandsOnOneConnection(t *testing.T) {
	addr := startServer(t, storeHandler())
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprint(conn, "PING\nSET a 0 1\nGET a\n") // pipelined: three frames in one write
	r := bufio.NewReader(conn)
	for _, want := range []string{"PONG\n", "OK\n", "VALUE 1\n"} {
		if got, err := r.ReadString('\n'); err != nil || got != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	}
}

func TestOversizedLineDoesNotKillServer(t *testing.T) {
	addr := startServer(t, storeHandler())
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	go conn.Write(bytes.Repeat([]byte("a"), maxLineBytes+10)) // no newline, ever
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	io.Copy(io.Discard, conn) // returns once the server hangs up on us
	conn.Close()

	if got, err := client.Do(addr, "PING", time.Second); err != nil || got != "PONG" {
		t.Fatalf("server unhealthy after oversized line: %q, %v", got, err)
	}
}

func TestConcurrentClients(t *testing.T) {
	addr := startServer(t, storeHandler())
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			key, val := "k"+strconv.Itoa(i), "v"+strconv.Itoa(i)
			if got, err := client.Do(addr, "SET "+key+" 0 "+val, time.Second); err != nil || got != "OK" {
				t.Errorf("SET %s -> %q, %v", key, got, err)
				return
			}
			if got, err := client.Do(addr, "GET "+key, time.Second); err != nil || got != "VALUE "+val {
				t.Errorf("GET %s -> %q, %v", key, got, err)
			}
		})
	}
	wg.Wait()
}

func TestCloseDisconnectsClientsAndStopsServe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(HandlerFunc(func(protocol.Command) string { return protocol.RespPong }))
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	fmt.Fprint(conn, "PING\n")
	if got, _ := r.ReadString('\n'); got != "PONG\n" { // ensures the conn is accepted and tracked
		t.Fatalf("got %q", got)
	}

	srv.Close()
	if err := <-served; err != nil {
		t.Fatalf("Serve returned %v after Close, want nil", err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := r.ReadString('\n'); err == nil {
		t.Fatal("idle client connection still open after Close")
	}
	srv.Close() // idempotent: must not panic or block
}
