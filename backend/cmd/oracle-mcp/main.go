package main

import (
	"errors"
	"log"
	"net/http"
	"opskeeper/backend/mcpserver/oracle/server"
	"os"
	"time"
)

func main() {
	cfg := server.ConfigFromEnv()
	cfg.Logger = log.New(os.Stdout, "", 0)
	h, e := server.New(cfg)
	if e != nil {
		cfg.Logger.Printf("%s [ERROR] build Oracle MCP server: %v", time.Now().Format(time.RFC3339), e)
		os.Exit(1)
	}
	if e = http.ListenAndServe(cfg.Address, h); e != nil && !errors.Is(e, http.ErrServerClosed) {
		cfg.Logger.Printf("%s [ERROR] Oracle MCP server stopped: %v", time.Now().Format(time.RFC3339), e)
		os.Exit(1)
	}
}
