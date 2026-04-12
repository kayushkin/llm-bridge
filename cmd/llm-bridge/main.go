package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kayushkin/llm-bridge/internal/config"
	"github.com/kayushkin/llm-bridge/internal/server"
	"github.com/kayushkin/llm-bridge/internal/store"
)

func main() {
	cfg := config.Load()

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer st.Close()

	srv := server.New(st)

	go func() {
		log.Printf("llm-bridge listening on %s", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nshutting down")
}
