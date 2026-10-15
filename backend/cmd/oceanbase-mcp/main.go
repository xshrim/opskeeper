package main

import (
	"errors"
	"log"
	"net/http"
	"opskeeper/backend/mcpserver/oceanbase/server"
	"os"
	"time"
)

func main() {
	cfg := server.ConfigFromEnv()
	cfg.Logger = log.New(os.Stdout, "", 0)
	h, err := server.New(cfg)
	if err != nil {
		cfg.Logger.Printf("%s [ERROR] build OceanBase MCP server: %v", time.Now().Format(time.RFC3339), err)
		os.Exit(1)
	}
	if err = http.ListenAndServe(cfg.Address, h); err != nil && !errors.Is(err, http.ErrServerClosed) {
		cfg.Logger.Printf("%s [ERROR] OceanBase MCP server stopped: %v", time.Now().Format(time.RFC3339), err)
		os.Exit(1)
	}
}
