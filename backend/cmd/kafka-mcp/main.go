package main

import (
	"errors"
	"log"
	"net/http"
	"opskeeper/backend/mcpserver/kafka/server"
	"os"
)

func main() {
	cfg := server.ConfigFromEnv()
	cfg.Logger = log.New(os.Stdout, "", 0)
	h, e := server.New(cfg)
	if e != nil {
		cfg.Logger.Printf("[ERROR] build Kafka MCP server: %v", e)
		os.Exit(1)
	}
	if e = http.ListenAndServe(cfg.Address, h); e != nil && !errors.Is(e, http.ErrServerClosed) {
		cfg.Logger.Printf("[ERROR] Kafka MCP server stopped: %v", e)
		os.Exit(1)
	}
}
