package main

import (
	"log"
	"net/http"
	"opskeeper/backend/mcpserver/minio/server"
	"os"
)

func main() {
	cfg := server.ConfigFromEnv()
	cfg.Logger = log.New(os.Stderr, "minio-mcp ", log.LstdFlags)
	h, err := server.New(cfg)
	if err != nil {
		cfg.Logger.Fatal(err)
	}
	cfg.Logger.Printf("listening on %s", cfg.Address)
	if err := http.ListenAndServe(cfg.Address, h); err != nil {
		cfg.Logger.Fatal(err)
	}
}
