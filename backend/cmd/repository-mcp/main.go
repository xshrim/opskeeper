package main

import (
	"log"
	"net/http"
	"opskeeper/backend/mcpserver/repository/server"
)

func main() {
	cfg := server.ConfigFromEnv()
	h, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("repository MCP listening on %s", cfg.Address)
	log.Fatal(http.ListenAndServe(cfg.Address, h))
}
