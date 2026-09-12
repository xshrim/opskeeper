package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"opskeeper/backend/mcpserver/postgresql/server"
)

func main() {
	cfg := server.ConfigFromEnv()
	cfg.Logger = log.New(os.Stdout, "", 0)
	h, err := server.New(cfg)
	if err != nil {
		cfg.Logger.Printf("%s [ERROR] build PostgreSQL MCP server: %v", time.Now().Format(server.LogTimeLayout), err)
		os.Exit(1)
	}
	server.LogStartup(cfg.Logger, cfg)
	if err := http.ListenAndServe(cfg.Address, h); err != nil && !errors.Is(err, http.ErrServerClosed) {
		cfg.Logger.Printf("%s [ERROR] PostgreSQL MCP server stopped: %v", time.Now().Format(server.LogTimeLayout), err)
		os.Exit(1)
	}
}
