package main

//want to take a search space and run
//all answers are saved and emitted to serf
//serf should contain:
//all answers met
//a list of all current work units (find the highest, increment one)
//a list of all completed work units, we truncate if we have all values lower then the lowest. otherwise just append
//a list of all completed work units with associated answers
//if a work unit is completed, a worker takes the next one and ups the count (this only works with etcd)
//all incoming answers are saved
//answers should contain a bookmark into what space it belongs

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
	"fmt"
)

const (
	rsize      = 6
	boundary   = 101
	tabooSize  = 500
	startSize  = 32
	bigCount   = 9999999
	ceDir      = "./counter-examples/"
	serverIP   = "http://128.111.84.156:6666/sync/"
	listenAddr = ":5555"
)

var (
	g             *Graph
	shouldPreempt chan bool
)

type Graph struct {
	G      []int
	Size   int
	Parity int
}

// random balanced graph
func newGraph(s int) *Graph {
	g := &Graph{Size: s, G: make([]int, s*s)}

	// upper triangle size
	var tsize int
	for i := s - 1; i > 0; i-- {
		tsize += i
	}
	g.Parity = tsize % 2
	// fill array with even 1s and 0s
	t := make([]int, tsize)
	for i := 0; i < len(t)/2; i++ {
		t[i] = 1
	}
	// shuffle upper triangle only
	perm := rand.Perm(tsize)
	var k int
	for i := 0; i < s; i++ {
		for j := i + 1; j < s; j++ {
			g.G[i*s+j] = t[perm[k]]
			k++
		}
	}
	return g
}

//balanced increment
func (g *Graph) inc() {
	ng := newGraph(g.Size + 1)
	for i := 0; i < g.Size; i++ {
		for j := 0; j < g.Size; j++ {
			ng.G[i*ng.Size+j] = g.G[i*g.Size+j]
		}
	}

	// Create new column of even 1s and 0s to balance graph
	newCol := make([]int, ng.Size-1)
	perm := rand.Perm(len(newCol))
	for i := 0; i < len(newCol)/2; i++ {
		newCol[i] = 1
	}
	// Are we adding an odd # of edges to a graph that is already odd? Balance.
	if g.Parity == 1 && len(newCol)%2 == 1 {
		newCol[len(newCol)-1] = 1
	}
	// Add permuted column
	for i := 0; i < len(newCol); i++ {
		ng.G[i*ng.Size+ng.Size-1] = newCol[perm[i]]
	}
	g.Size = ng.Size
	g.G = ng.G
	if len(newCol)%2 == 1 {
		g.Parity = (g.Parity + 1) % 2
	}
}
func (g *Graph) String() string {
	var s string
	for i := 0; i < g.Size; i++ {
		for j := 0; j < g.Size; j++ {
			s += strconv.Itoa(g.G[i*g.Size+j])
			s += " "
		}
		s += "\n"
	}
	return s
}
func (g *Graph) toFile() {
	filename := ceDir + "ce" + strconv.Itoa(g.Size) + ".txt"
	log.Println("writing: " + filename)
	count := 0
	for i := 0; i < g.Size; i++ {
		for j := i + 1; j < g.Size; j++ {
			if g.G[i*g.Size+j] == 1 {
				count++
			} else {
				count--
			}
		}
	}
	log.Println("Graph parity is", count)

	file, err := os.Create(filename)
	if err != nil {
		log.Println(err)
	}

	enc := gob.NewEncoder(file)
	err = enc.Encode(*g)
	if err != nil {
		log.Println(err)
	}

	file.Close()
}

func (g *Graph) postToVault() {
	type postData struct {
		Solution        string `json:"solution"`
		ClientTimestamp int64  `json:"clientTimestamp"`
	}

	data := postData{Solution: g.graphString(), ClientTimestamp: (time.Now().UnixNano() / int64(time.Millisecond))}
	b, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
	}

	log.Println("Sending counter example")
	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://richcoin.cs.ucsb.edu:8280/vault/1.0.0", bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Authorization", "Bearer Gs117zVTDYHUXf9HkZuUE4XKME0a")
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)

	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	buffer := bytes.NewBuffer(body)
	log.Println("%v", buffer.String())

	log.Println("Response:")
	log.Println(resp)
}

func (g *Graph) graphString() string {
	var s string
	for i := 0; i < g.Size*g.Size; i++ {
		s += strconv.Itoa(g.G[i])
	}
	return s
}

//initialize with smaller graph (random optional)
//random initialize

func cliqueCount(g *Graph) int {
	var i, j, k, l, m, n int
	var count int

	for i = 0; i < g.Size-rsize+1; i++ {
		for j = i + 1; j < g.Size-rsize+2; j++ {
			for k = j + 1; k < g.Size-rsize+3; k++ {
				if (g.G[i*g.Size+j] == g.G[i*g.Size+k]) &&
					(g.G[i*g.Size+j] == g.G[j*g.Size+k]) {
					for l = k + 1; l < g.Size-rsize+4; l++ {
						if (g.G[i*g.Size+j] == g.G[i*g.Size+l]) &&
							(g.G[i*g.Size+j] == g.G[j*g.Size+l]) &&
							(g.G[i*g.Size+j] == g.G[k*g.Size+l]) {
							for m = l + 1; m < g.Size-rsize+5; m++ {
								if (g.G[i*g.Size+j] == g.G[i*g.Size+m]) &&
									(g.G[i*g.Size+j] == g.G[j*g.Size+m]) &&
									(g.G[i*g.Size+j] == g.G[k*g.Size+m]) &&
									(g.G[i*g.Size+j] == g.G[l*g.Size+m]) {
									for n = m + 1; n < g.Size-rsize+6; n++ {
										if (g.G[i*g.Size+j] == g.G[i*g.Size+n]) &&
											(g.G[i*g.Size+j] == g.G[j*g.Size+n]) &&
											(g.G[i*g.Size+j] == g.G[k*g.Size+n]) &&
											(g.G[i*g.Size+j] == g.G[l*g.Size+n]) &&
											(g.G[i*g.Size+j] == g.G[m*g.Size+n]) {
											count++
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return (count)
}

func speculativeFlip(x1, x2 int, g *Graph, tabooList map[node]struct{}, wg *sync.WaitGroup, results chan node) {
	var bestNode = node{}
	var i, j, k, l int
	var count int
	bestCount := bigCount
	bestNode.count = bigCount

	defer wg.Done()
	//divide up i to g.size range, and give a copy of the graph, we return best count
	for i = x1; i < x2; i++ {
		for j = i + 1; j < g.Size; j++ {
			//for k = 0; k < g.Size && flag == 0; k++ {
			//for l = i + 1; l < g.Size && flag == 0; l++ {

			for {
				k = rand.Intn(g.Size - 1)
				if k == i {
					if g.Size-k-1 == 1 {
						continue
					}
					l = rand.Intn(g.Size-k-2) + k + 1
					if l >= j {
						l += 1
					}
				} else {
					//calculate first position that is legal in row
					l = rand.Intn(g.Size-k-1) + k + 1
				}

				//make sure flipping opposide edges
				if g.G[i*g.Size+j] != g.G[k*g.Size+l] {
					break
				}
			}
			//YAY flip edges
			g.G[i*g.Size+j] = 1 - g.G[i*g.Size+j]
			g.G[k*g.Size+l] = 1 - g.G[k*g.Size+l]

			count = cliqueCount(g)

			//order our edge flips
			var cnode node
			if i > k || (i == k && j > l) {
				cnode = node{k, l, i, j, count}
			} else {
				cnode = node{i, j, k, l, count}
			}
			//Is it better and the i,j,count not taboo?
			_, ok := tabooList[cnode]
			if count < bestCount && !ok {
				bestCount = count
				bestNode = cnode
			}

			//Flip both back
			g.G[i*g.Size+j] = 1 - g.G[i*g.Size+j]
			g.G[k*g.Size+l] = 1 - g.G[k*g.Size+l]

			if count == 0 {
				results <- cnode
				return
			}
		}
	}
	results <- bestNode
	return
}

type node struct {
	i, j, k, l int
	count      int
}

func makePrimeGen(i int) func() int {
	if i%2 == 0 {
		i++
	}
	return func() int {
		i += 2
		for !isPrime(i) {
			i += 2
		}
		return i
	}
}
func isPrime(i int) bool {
	for j := 2; j <= i/2; j++ {
		if i%j == 0 {
			return false
		}
	}
	return true
}

/**********************
    Preemption code
**********************/

func registerPreemptionHandler() {
	log.Println("Registering preemption handler")
	r := mux.NewRouter()
	r.HandleFunc("/preempt/", errorHandler(preemptionHandler)).Methods("POST")
	http.Handle("/", r)
}

func preemptionHandler(w http.ResponseWriter, r *http.Request) error {
	log.Println("Preemption message received")
	var receivedGraph *Graph = new(Graph)
	if err := json.NewDecoder(r.Body).Decode(receivedGraph); err != nil {
		return badRequest{err}
	}

	//Make sure received graph is valid
	if receivedGraph.Size*receivedGraph.Size != len(receivedGraph.G) {
		return badRequest{errors.New("Invalid preepmtion graph")}
	}

	shouldPreempt <- true

	return json.NewEncoder(w).Encode("OK")
}

// badRequest is handled by setting the status code in the reply to StatusBadRequest.
type badRequest struct{ error }

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

/******************
    Client code
******************/

func bootstrap(g *Graph) {
	log.Println("Bootstrapping")

	//Clear out all saved counter example files from last run
	//os.RemoveAll(ceDir)
	os.Mkdir(ceDir, 0777)

	//Seed with counter example file if given
	if len(os.Args) == 2 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			log.Fatal("Opening File", err)
		}
		dec := gob.NewDecoder(file)
		err = dec.Decode(g)
		if err != nil {
			log.Fatal("In decode:", err)
		}

		log.Println("Sucessfully loaded graph")
	}

	syncRequest(g)
}

//Send current graph and return true if we recieve new graph
func syncRequest(g *Graph) bool {
	log.Println("Syncing with server")

	b, err := json.Marshal(g)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", serverIP, bytes.NewReader(b))

	if err != nil {
		log.Fatal(err)
	}

	var resp *http.Response

	resp, err = client.Do(req)

	if err != nil {
		log.Println(err)
		//log.Println("Sleeping and trying again...")
		//time.Sleep(5 * time.Second)

		log.Println("Continuing search")
		return false
	}

	//Parse response, set graph to received one if necessary, and then start searching
	log.Println("Received response from server")

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var receivedGraph *Graph = new(Graph)
	err = json.Unmarshal(body, receivedGraph)
	if err != nil {
		log.Fatal(err)
	}

	//Make sure received graph is valid
	if receivedGraph.Size*receivedGraph.Size != len(receivedGraph.G) {
		log.Println("Received invalid graph")
		log.Println("Using own CE")
		return false
	}

	//Case 1: received graph is new CE to use (=> use received graph)
	//Case 2: received graph is empty and g is valid CE (=> Use g)
	//Case 3: received graph is empty and g is empty (=> Generate new graph)

	if receivedGraph.Size > 0 && len(receivedGraph.G) > 0 {
		log.Println("Using received graph")
		g.Size = receivedGraph.Size
		g.G = receivedGraph.G
		return true
	} else if (receivedGraph.Size == 0 && len(receivedGraph.G) == 0) && (g.Size > 0 && len(g.G) > 0) {
		log.Println("Using own CE")
		return false
	} else if (g.Size == 0 && len(g.G) == 0) && (receivedGraph.Size == 0 && len(receivedGraph.G) == 0) {
		log.Println("Using new graph")
		g = newGraph(startSize)
		return false
	} else {
		log.Println("Didn't match one of the cases!")
		log.Println("g: %v", g)
		log.Println("receivedGraph: %v", receivedGraph)
		log.Fatal("Quitting")
	}
	panic("unreachable")
	return false
}

func ramseySearch(g *Graph) {
	var lastg *Graph = newGraph(startSize)
	var primeGen func() int

	var count int
	var tabooList map[node]struct{}

	primeGen = makePrimeGen(g.Size)
	nsize := primeGen()

	//Make a new graph with size of next prime number
	for i := g.Size; i < nsize; i++ {
		lastg.Size = g.Size
		lastg.G = g.G
		g.inc()
	}

	tabooList = make(map[node]struct{})

	//If the best count decreases 3 times, re-randomize. This prevents all the backwards steps
	//TODO reimplement backoff

	//While we do not have a publishable result
	for g.Size <= boundary {
		//Check if we need to preempt
		select {
		case <-shouldPreempt:
			_ = syncRequest(g)
		default:
		}

		//Find out how we are doing
		count = cliqueCount(g)

		//If we have a counter example
		if count == 0 {
			log.Println("Eureka!  Counter-example found!")
			log.Println(g)

			//Save the counter example
			g.toFile()

			isUpdated := syncRequest(g)
			fmt.Println(isUpdated)

			if !isUpdated {
				//Make a new graph with size of next prime number
				primeGen = makePrimeGen(g.Size)
				nsize = primeGen()
				for i := g.Size; i < nsize; i++ {
					lastg.Size = g.Size
					lastg.G = g.G
					g.inc()
				}
			}
		}

		const numGo = 32
		var wg sync.WaitGroup
		var results = make(chan node, numGo)
		chunk := g.Size / numGo
		for i := 0; i < numGo; i++ {
			ng := newGraph(g.Size)
			copy(ng.G, g.G)
			wg.Add(1)

			x2 := i*chunk + chunk
			if i == numGo-1 {
				x2 = g.Size
			}
			go speculativeFlip(i*chunk, x2, ng, tabooList, &wg, results)
		}
		wg.Wait()
		close(results)

		bestNode := node{count: bigCount}
		for i := range results {
			if bestNode.count > i.count {
				bestNode = i
			}
		}

		//TODO new backoff algorithm

		if bestNode.count == bigCount {
			log.Println("No best edge found. Reverting to last counter-example")
			g.Size = lastg.Size
			g.G = lastg.G
			g.inc()
			tabooList = make(map[node]struct{})

			//Reset the backwards counter
			//bestBestCount = bigCount

			continue
		}

		//Keep the best flip we saw
		g.G[bestNode.i*g.Size+bestNode.j] = 1 - g.G[bestNode.i*g.Size+bestNode.j]
		g.G[bestNode.k*g.Size+bestNode.l] = 1 - g.G[bestNode.k*g.Size+bestNode.l]

		//Taboo this graph configuration so that we don't visit it again
		//count = cliqueCount(g)
		tabooList[bestNode] = struct{}{}

		log.Printf("ce size: %v, best_count: %v\n", g.Size, bestNode.count)

		//repeat!
	}
}

func server() {
	//Listen for preemption
	registerPreemptionHandler()

	err := http.ListenAndServe(listenAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}
func main() {
	g = new(Graph)
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(runtime.NumCPU())

	go server()

	//Initialize
	bootstrap(g)
	shouldPreempt = make(chan bool)

	//Run search and sync
	ramseySearch(g)
}
