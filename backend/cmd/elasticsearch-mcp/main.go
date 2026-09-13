package main

import (
	"errors"
	"log"
	"net/http"
	s "opskeeper/backend/mcpserver/elasticsearch/server"
	"os"
)

func main() {
	c := s.ConfigFromEnv()
	c.Logger = log.New(os.Stdout, "", 0)
	h, e := s.New(c)
	if e != nil {
		c.Logger.Fatal(e)
	}
	if e = http.ListenAndServe(c.Address, h); e != nil && !errors.Is(e, http.ErrServerClosed) {
		c.Logger.Fatal(e)
	}
}
