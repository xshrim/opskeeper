package main

import (
	"errors"
	"log"
	"net/http"
	"opskeeper/backend/mcpserver/rabbitmq/server"
	"os"
)

func main() {
	cfg := server.ConfigFromEnv()
	cfg.Logger = log.New(os.Stdout, "", 0)
	h, e := server.New(cfg)
	if e != nil {
		cfg.Logger.Printf("[ERROR] build RabbitMQ MCP server: %v", e)
		os.Exit(1)
	}
	if e = http.ListenAndServe(cfg.Address, h); e != nil && !errors.Is(e, http.ErrServerClosed) {
		cfg.Logger.Printf("[ERROR] RabbitMQ MCP server stopped: %v", e)
		os.Exit(1)
	}
}
