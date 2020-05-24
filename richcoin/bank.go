package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

const (
	offset   = 99
	max      = 101
	isoCheck = "./iso_check"
	bankFile = "bank.gob"
)

var b Bank

type Bank [][]Graph //index 0 is for solutions of 79

/*
type Coin struct {
	Solution []int
	Size     int
	Name     string
}
*/

func (c Graph) String() string {
	s := strconv.Itoa(c.Size) + " 0\n"
	var j int
	for i := 0; i < c.Size; i++ {
		for j = 0; j < c.Size-1; j++ {
			s += strconv.Itoa(c.G[i*c.Size+j]) + " "
		}
		s += strconv.Itoa(c.G[i*c.Size+j]) + "\n"
	}
	return s
}
func (c *Graph) name() {
	c.Name = strconv.Itoa(c.Size) + "_" + strconv.Itoa(len(b[c.Size-offset])) + ".state"
}


// True for sucessful addition to bank
// False if isomorphic to other solutions
func (b Bank) AddCoin(c Graph) bool {
	c.name()
	if checkIso(c) == false {
		b[c.Size-offset] = append(b[c.Size-offset], c)
		return true
	}
	return false
}

func (b *Bank) Save() error {
	file, err := os.Create(bankFile)
	if err != nil {
		log.Fatal("Creating save file:", err)
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	err = enc.Encode(b)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
func (b *Bank) load() {
	file, err := os.Open(bankFile)
	if err != nil {
		log.Println("Opening bank:", err)
		return
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	err = dec.Decode(b)
	if err != nil {
		log.Fatal(err)
	}
}
func flushFiles(b Bank) {
	for i := range b {
		for j := range b[i] {
			_ = flushFile(b[i][j])
		}
	}
}
func flushFile(c Graph) error {

	file, err := os.Create(c.Name)
	if err != nil {
		log.Println("flushFile:", err)
	}
	fmt.Fprintf(file, "%s", c)
	file.Close()
	return err
}

func NewBank() Bank {
	b := make([][]Graph, max-offset+1)
	for i := 0; i < len(b); i++ {
		b[i] = make([]Graph, 0)
	}
	return b
}

func checkIso(c Graph) bool {
	err := flushFile(c)
	if err != nil {
		log.Println(err)
		return true
	}
	for i := 0; i < len(b[c.Size-offset]); i++ {
		out, err := exec.Command(isoCheck, "-g", "/home/patch/go/src/richcoin/phase2/" + c.Name, "-f", "/home/patch/go/src/richcoin/phase2/" + b[c.Size-offset][i].Name).CombinedOutput()
		fmt.Println("-g /home/patch/go/src/richcoin/" + c.Name + " -f /home/patch/go/src/richcoin/" + b[c.Size-offset][i].Name)
		if err != nil {
			log.Fatal("exec:", err, string(out))
		}
		fmt.Println("Output:", string(out))
		if string(out[:3]) == "YES" {
			return true
		} else if string(out[:2]) == "NO" {
			continue
		} else {
			log.Println("error reading input", out)
			return true
		}
	}
	return false
}

func init() {
	if _, err := os.Stat(isoCheck); os.IsNotExist(err) {
		log.Fatal("Error opening ", isoCheck)
		return
	}
	b = NewBank()
	b.load()
	flushFiles(b)
}
/*
func main() {
	c1 := Coin{Size: 4, Solution: []int{0, 1, 0, 1, 0, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0}}
	c2 := Coin{Size: 4, Solution: []int{0, 1, 0, 1, 0, 0, 0, 1, 0, 1, 1, 1, 1, 0, 0, 0}}
	c3 := Coin{Size: 4, Solution: []int{1, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 1, 1, 0, 0, 0}}
	c4 := Coin{Size: 4, Solution: []int{0, 1, 0, 1, 0, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0}}
	fmt.Println(b.AddCoin(c1))
	fmt.Println(b.AddCoin(c2))
	fmt.Println(b.AddCoin(c3))
	fmt.Println(b.AddCoin(c4))
	fmt.Println("%#+v",b)
	fmt.Println(len(b))
	fmt.Println(b[0][1])
}
*/
