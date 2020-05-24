//This is the centralized server that keeps track of a group of registered
//ramsey miners. Miners are tracked by IP and current ramsey number. Status of
//these miners are updated upon registration, re-registration(after a miner
//re-starts), and upon a miner sending a newly generated counter example. The
//scheduler will also pre-empt clients upon recieving a new higher counter
//example. In future versions the scheduler will be in charge of issueing
//different clients to use different methods (cyclic vs two-flip taboo) on
//different seed values. Currently it makes all clients work on the next highest
//prime ramsey number (sadly this dosen't seem to help that much with the cyclic
//client being offline)
package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"strings"

	"github.com/gorilla/mux"
)

const (
	listenAddr = ":6666"
	indexFile  = "ce.gob"
)

var (
	db        = newDB()
	latest    = graphStore{G: newGraph()}
)

// thread-safe map of IP to current working ce
// http://www.youtube.com/watch?v=2-pPAvqyluI
type nameStore struct {
	datab map[string]int
	mu    sync.RWMutex
}

func (s *nameStore) Get(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.datab[key]
}

func (s *nameStore) Set(key string, value int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, present := s.datab[key]
	if present {
		return false
	}
	s.datab[key] = value
	return true
}
func (s *nameStore) Delete(key string) {
	s.mu.Lock()
	delete(s.datab, key)
	s.mu.Unlock()
}
func (s *nameStore) Update(key string, value int) {
	s.mu.Lock()
	delete(s.datab, key)
	s.datab[key] = value
	s.mu.Unlock()
}

type name struct {
	k string
	v int
}

func (s *nameStore) GetList() []name {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]name, 0)
	for k, v := range s.datab {
		list = append(list, name{k, v})
	}
	return list
}
func newDB() *nameStore {
	return &nameStore{
		datab: make(map[string]int),
	}
}

type graph struct {
	G    []int
	Size int
}

func newGraph() graph {
	return graph{G: make([]int, 0)}
}

type graphStore struct {
	G  graph
	mu sync.RWMutex
}

func (g *graphStore) Size() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.G.Size
}
func (g *graphStore) Get() graph {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.G
}
func (g *graphStore) Update(n graph) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n.Size >= g.G.Size {
		g.G = n
	} else {
		log.Println("Avoided a race condidtion I didn't think I'd see in practice!")
	}
}

func (g *graphStore) Save() {
	g.mu.Lock()
	defer g.mu.Unlock()

	file, err := os.Create(indexFile)
	if err != nil {
		log.Fatal("Creating save file:", err)
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	err = enc.Encode(g)
	if err != nil {
		log.Fatal(err)
	}
}
func (g *graphStore) Load() {
	g.mu.Lock()
	defer g.mu.Unlock()

	file, err := os.Open(indexFile)
	if err != nil {
		log.Println("Opening latest ce:", err)
		return
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	err = dec.Decode(g)
	if err != nil {
		log.Fatal(err)
	}
}

//Expects a graph counter example:
// --If greater then or equal to latest send back empty
// --If less then latest send back latest
func syncHandler(w http.ResponseWriter, r *http.Request) error {
	//get counter example
	req := newGraph()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest{err}
	}
	//register IP and current counter example
	i := strings.Index(r.RemoteAddr, ":")
	remote :=  r.RemoteAddr[:i]
	if db.Set(remote, req.Size) {
		log.Println("Registered new client: ", remote, ":", req.Size)
	} else {
		s := db.Get(remote)
		if s > req.Size {
			log.Println("Re-register client: ", remote, ":", req.Size)
		} else {
			//log.Println("Client sync from:", r.RemoteAddr)
		}
		db.Update(remote, req.Size)
	}

	var resp = newGraph()
	if req.Size < latest.Size() {
		log.Println("Recieved sync less then latest, sending latest")
		resp = latest.Get()
	} else if req.Size == latest.Size() {
		log.Println("Recieved sync == latest")
		latest.Update(req)
		latest.Save()
	} else if req.Size != 0 {
		log.Println("Recieved sync > latest. Latest set at: ", req.Size)
		latest.Update(req)
		latest.Save()
		go preEmpt(req.Size)
	}
	return json.NewEncoder(w).Encode(resp)
}

//Sends latest graph to everyone registered. If anyone has an error they are
//de-registered.
var pGuard sync.Mutex
const PathPrefix = "/preempt/"

func preEmpt(initSize int) {
	log.Println("Sending out premptions for size:", initSize)
	pGuard.Lock()
	defer pGuard.Unlock()

	g := latest.Get()
	list := db.GetList()
	if initSize < g.Size {
		log.Println("Very unexpected preEmpt race")
		return
	}

	b, err := json.Marshal(g)
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range list {
		//client is already working on the next size so don't send
		if c.v > g.Size {
			continue
		}
		client := &http.Client{}
		req, err := http.NewRequest("POST", "http://" + c.k + ":5555"+ PathPrefix, bytes.NewReader(b))
		if err != nil {
			log.Fatal(err)
		}
		_, err = client.Do(req)
		//delete registered people who don't respond
		if err != nil {
			log.Println("Error reaching client ", c.k, ":", err)
			db.Delete(c.k)
		}
		db.Update(c.k, g.Size)
	}
	log.Println("Finished premptions msgs for size:", initSize)
}

// badRequest is handled by setting the status code in the reply to StatusBadRequest.
type badRequest struct{ error }

// errorHandler wraps a function returning an error by handling the error and returning a http.Handler.
// If the error is of the one of the types defined above, it is handled as described for every type.
// If the error is of another type, it is considered as an internal error and its message is logged.
func errorHandler(f func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := f(w, r)
		if err == nil {
			return
		}
		switch err.(type) {
		case badRequest:
			log.Println("Bad Request: ", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			log.Println(err)
			http.Error(w, "oops", http.StatusInternalServerError)
		}
	}
}

func registerHandlers() {
	r := mux.NewRouter()
	r.HandleFunc("/sync/", errorHandler(syncHandler)).Methods("POST")
	http.Handle("/", r)
}

func safeExit(c chan os.Signal) {
	s := <-c
	log.Println("Recieved signal", s, "saving and exiting")
	latest.Save()
	os.Exit(0)
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	go safeExit(c)

	logFile, err := os.Create("log.out")
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logFile)

	latest.Load()
	registerHandlers()

	log.Println("Latest CE is of size:", latest.Size())
	log.Println("Starting Server")
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
