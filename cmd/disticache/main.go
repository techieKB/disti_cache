// Command disticache runs a single DistiCache node.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/techieKB/disti_cache/internal/node"
	"github.com/techieKB/disti_cache/internal/protocol"
	"github.com/techieKB/disti_cache/internal/server"
	"github.com/techieKB/disti_cache/internal/store"
)

func main() {
	addr := flag.String("addr", ":7000", "TCP listen address")
	shards := flag.Int("shards", 16, "lock shards in the local store")
	capacity := flag.Int("capacity", 10000, "max keys per shard")
	flag.Parse()

	st := store.New(*shards, *capacity)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go st.RunJanitor(ctx, time.Second)

	srv := server.New(server.HandlerFunc(func(c protocol.Command) string { return node.Execute(st, c) }))
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	log.Printf("listening on %s", ln.Addr())
	if err := srv.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
