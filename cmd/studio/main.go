package main

import (
	"context"
	"flag"
	"fmt"
	coreserver "local/l2dagent/CoreServer"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:17823", "loopback listen address")
	data := flag.String("data", ".local", "local workspace data directory")
	web := flag.String("web", "CoreServer/web/dist", "built frontend directory")
	flag.Parse()
	if e := os.MkdirAll("Live2DCubism", 0755); e != nil {
		log.Fatalf("create Live2DCubism directory: %v", e)
	}
	host, _, e := net.SplitHostPort(*addr)
	if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("addr must contain a loopback IP and port")
	}
	app, e := coreserver.New(*data, *web)
	if e != nil {
		log.Fatal(e)
	}
	server := &http.Server{Addr: *addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	fmt.Printf("人人皆可建模: http://%s\n", *addr)
	if e = server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
