package search

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"time"
)

var index Index

type DocTerm struct {
	Term      string
	Pack      string
	Path      string
	Functions int
	Imports   int
	Packages  int
	Types     int
	//Comments  int
}

//map full pkg paths to docterm data
type DocMap map[string]*DocTerm

func (d DocMap) String() string {
	var pretty string
	pretty += fmt.Sprintln("")
	for k, v := range d {
		pretty += fmt.Sprintln("        ", k, ": ", v)
	}
	return pretty
}

type Index struct {
	Index      map[string]DocMap
	UniquePkgs int
}

func OpenIndex(indexFile string) {
	log.Println("Reading index file...")
	if *specific {
		log.Println("Srank enabled")
	}
	file, err := os.Open(indexFile)
	if err != nil {
		log.Fatal(err)
	}
	t0 := time.Now()
	dec := gob.NewDecoder(file)
	dec.Decode(&index)
	t1 := time.Now()
	log.Printf("Read in index of size %v\n", len(index.Index))
	log.Printf("Decoding took %v\n", t1.Sub(t0))
}
