package main

import (
	"errors"
	"log"
	"net/http"
	"opskeeper/backend/mcpserver/mysql/server"
	"os"
)

func main() {
	cfg := server.ConfigFromEnv()
	cfg.Logger = log.New(os.Stdout, "", 0)
	h, err := server.New(cfg)
	if err != nil {
		cfg.Logger.Printf("[ERROR] build MySQL MCP server: %v", err)
		os.Exit(1)
	}
	if err = http.ListenAndServe(cfg.Address, h); err != nil && !errors.Is(err, http.ErrServerClosed) {
		cfg.Logger.Printf("[ERROR] MySQL MCP server stopped: %v", err)
		os.Exit(1)
	}
}
