package main

import (
	"bytes"
	"sort"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

const saveFile = "usedCoins.gob"

var tokens = []string{"MlXl7zpdz86zp3zbWpVCUoWqRWka", "CI4hp9fTsZwfp4lbYOE82shGMOYa", "i7OAj5BOiTP2eCMAGkN4CrfnoFQa", "3hdOfh7fg4lfRAg3NZsl6GHpWd4a"}
var seenCoins = map[string]struct{}{}
var rateLimit = time.Tick(775 * time.Millisecond)
var m sync.Mutex

// Fetch coins from vault, place in newCoins
type BySize []Coin
func (a BySize) Len() int           { return len(a) }
func (a BySize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySize) Less(i, j int) bool { return a[i].Size < a[j].Size }

func fetchCoinsFromVault() {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://richcoin.cs.ucsb.edu:8280/vault/1.0.0", nil)
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

	var coins []Coin
	err = json.Unmarshal(buffer.Bytes(), &coins)
	if err != nil {
		log.Println("error:", err)
	}

	sort.Sort(sort.Reverse(BySize(coins)))
	for _, coin := range coins {
		_, ok := seenCoins[coin.CoinID]
		if !ok {
			<-rateLimit
			err = coin.fetchSolution()
			if err != nil {
				continue
			}
 			g := new(Graph)
			g.G = translateSolution([]byte(coin.Solution))
			g.Size = coin.Size
			if g.Size*g.Size != len(g.G) {
				log.Println("BAD GRAPH WTF!!!!")
				log.Println(coin.CoinID)
				return
			}
			if g.Size <99 {
				log.Fatal("coin is small?", coin.CoinID)
			}
			if b.AddCoin(*g) == false {
				log.Fatal("Unseen coin from bank isomorph?")
			}
			fmt.Println("added coin id:", coin.CoinID)

			m.Lock()
			seenCoins[coin.CoinID] = struct{}{}
			m.Unlock()
			//submitDecGraphs(coin)
		}
	}
	findNewSolutions(b[2][0])
}

type Coin struct {
	CoinID   string `json:"coinID"`
	Solution string `json:"solution"`
	Size     int    `json:"size"`
}

func (c *Coin) fetchSolution() error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://richcoin.cs.ucsb.edu:8280/vault/1.0.0/"+c.CoinID, nil)
	if err != nil {
		log.Fatal(err)
		return err
	}

	token := getToken()
	fmt.Println("Token:", token)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)

	if err != nil {
		log.Println(err)
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	buffer := bytes.NewBuffer(body)

	err = json.Unmarshal(buffer.Bytes(), c)
	if err != nil {
		log.Println("error:", err)
		return err
	}
	return nil
}

type Graph struct {
	G    []int
	Size int
	Name string
}

func newGraph(s int) *Graph {
	return &Graph{Size: s, G: make([]int, s*s)}
}
/*
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
*/
func (g *Graph) dec(node int) {
	//node := rand.Intn(g.Size)
	// del row
	//row := g.Size - 1
	col := node
	for i := 0; i < g.Size; i++ {
		g.G[i*g.Size+col] = 2
	}
	// del  col
	//col := g.Size - 1
	row := node
	for j := 0; j < g.Size; j++ {
		g.G[row*g.Size+j] = 2
	}
	//fmt.Println(g)
	//copy graph
	ng := newGraph(g.Size - 1)
	k := 0
	for i := 0; i < g.Size; i++ {
		for j := 0; j < g.Size; j++ {
			if g.G[i*g.Size+j] != 2 {
				ng.G[k] = g.G[i*g.Size+j]
				k++
			}

		}
	}
	g.Size = ng.Size
	g.G = ng.G
	//copy(g.G, ng.G)
	//g.G = g.G[:len(ng.G)]

	if g.Size*g.Size != len(g.G) {
		log.Fatal("WTF LENGTH")
	}
}
func (g *Graph) postToVault() error {
	type postData struct {
		Solution        string `json:"solution"`
		ClientTimestamp int64  `json:"clientTimestamp"`
	}

	data := postData{Solution: g.graphString(), ClientTimestamp: (time.Now().UnixNano() / int64(time.Millisecond))}
	b, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return  err
	}

	fmt.Println("Sending counter example")
	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://richcoin.cs.ucsb.edu:8280/vault/1.0.0", bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
		return  err
	}
	token := getToken()
	fmt.Println("Token:", token)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)

	if err != nil {
		log.Println(err)
		return  err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	buffer := bytes.NewBuffer(body)
	c := new(Coin)

	err = json.Unmarshal(buffer.Bytes(), c)
	if err != nil {
		log.Println("error:", err)
		return  err
	}
	fmt.Println(c.CoinID)
	fmt.Println("Response:")
	fmt.Println(resp)

	return nil
}

func (g *Graph) graphString() string {
	var s string
	for i := 0; i < g.Size*g.Size; i++ {
		s += strconv.Itoa(g.G[i])
	}
	return s
}

func findNewSolutions(g Graph) {

	//fmt.Println(g)
	//seed := newGraph(g.Size)
	//seed.Size = g.Size
	//copy(seed.G, g.G)

	var node101, node100 int
	count := 0
	for node101 = 0; node101 <101; node101++ {
		for node100 = 0; node100 <100; node100++ {
			g2 := newGraph(g.Size)
			copy(g2.G, g.G)
			g2.dec(node101)

			if b.AddCoin(*g2) {
				fmt.Println("Got one!!!!", g2.Name)
			}
			count++
			g2.dec(node100)

			if b.AddCoin(*g2) {
				fmt.Println("Got one!!!!", g2.Name)
			}
			count++
			fmt.Println(count)
		}
	}
	log.Println("All done, did you find anything?", count)
}
/*
func submitDecGraphs(c Coin) {
	g := new(Graph)
	g.G = translateSolution([]byte(c.Solution))
	g.Size = c.Size
	if g.Size*g.Size != len(g.G) {
		saveSeenCoins()
		log.Println("BAD GRAPH WTF!!!!")
		log.Println(c.CoinID)
		return
	}

	seed := newGraph(g.Size)
	seed.Size = g.Size
	copy(seed.G, g.G)

	for {
		if g.Size <= 99 {
			return
		}

			g = newGraph(seed.Size)
			copy(g.G, seed.G)

			if g.Size*g.Size != len(g.G) {
				saveSeenCoins()
				log.Println("BAD GRAPH after resetting seed")
				log.Println(c.CoinID)
			}
		}
		g.dec()
		j := cliqueCount(g)
		if j == 0 {
			fmt.Println("Decremented to:", g.Size)
			<-rateLimit
			err := g.postToVault()
			if err != nil {
				fmt.Println(err)
			}
		} else {
			fmt.Println("Error!, Clique count is not zero:", j)
		}
	}
}
*/

func translateSolution(s []byte) []int {
	g := make([]int, len(s))
	for i := range s {
		if s[i] == 48 {
			g[i] = 0
		} else {
			g[i] = 1
		}
	}
	return g
}

func saveSeenCoins() {
	file, err := os.Create(saveFile)
	if err != nil {
		log.Fatal("Creating save file:", err)
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	err = enc.Encode(seenCoins)
	if err != nil {
		fmt.Println(err)
	}
}

func safeExit(c chan os.Signal) {
	s := <-c
	fmt.Println("Recieved signal", s, "saving and exiting")
	m.Lock()
	saveSeenCoins()
	m.Unlock()
	os.Exit(0)
}

var tcount = 0

func getToken() string {
	tcount = (tcount + 1) % len(tokens)
	return tokens[tcount]
}

func main() {
	file, err := os.Create("./log.txt")
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)

	if len(os.Args) == 2 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			log.Fatal("Opening seen coin file", err)
		}
		defer file.Close()

		dec := gob.NewDecoder(file)
		err = dec.Decode(&seenCoins)
		if err != nil {
			log.Fatal("Deserialize seen file:", err)
		}
	}

	rand.Seed(time.Now().UnixNano())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	go safeExit(c)

		fetchCoinsFromVault()
		saveSeenCoins()
		log.Println("Apparently submitted all coins, recycling")
		time.Sleep(10 * time.Second)
}

func cliqueCount(g *Graph) int {
	var i, j, k, l, m, n int
	var count int
	rsize := 6

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
