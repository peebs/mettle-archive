// A stand-alone HTTP server providing a web UI for task management.
package main

import (
	"log"
	"net/http"
	"flag"

	"go-search/server"
	"go-search/search"
)

const (
	listenAddr = ":8000"
	indexFile = "./parser/index.gob"
)

func main() {
	flag.Parse()
    search.OpenIndex(indexFile)

	server.RegisterHandlers()
	http.Handle("/", http.FileServer(http.Dir("static")))
	log.Println("Listening at", listenAddr)
	http.ListenAndServe(listenAddr, nil)
}
