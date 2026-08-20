package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"opskeeper/backend/mcpserver/docker/server"
)

func main() {
	cfg := server.ConfigFromEnv()
	logger := log.New(os.Stdout, "", 0)
	cfg.Logger = logger
	handler, err := server.New(cfg)
	if err != nil {
		logger.Printf("%s [ERROR] build Docker MCP server: %v", time.Now().Format(server.LogTimeLayout), err)
		os.Exit(1)
	}

	server.LogStartup(logger, cfg)
	if err := http.ListenAndServe(cfg.Address, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Printf("%s [ERROR] Docker MCP server stopped: %v", time.Now().Format(server.LogTimeLayout), err)
		os.Exit(1)
	}
}
