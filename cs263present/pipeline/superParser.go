package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"flag"
)

const (
	indexFile     = "./index.gob"
	numDirParsers = 20
)

var (
	index Index = Index{Index: make(IndexMap)}
	commentParse = flag.Bool("c", false, "Parse with comments?")
	inputPath = flag.String("in", "", "Input file to parse")
)

type DocTerm struct {
	Term      string
    Pack      string
	Path      string //github import path -- should work with go get
	Functions int
	Imports   int
	Packages  int
	Types     int
}
type DocMap map[string]*DocTerm

func (d DocMap) String() string {
	var pretty string
	pretty += fmt.Sprintln("")
	for k, v := range d {
		pretty += fmt.Sprintln("        ", k, ": ", v)
	}
	return pretty
}

type IndexMap map[string]DocMap

type Index struct {
	Index      IndexMap
	UniquePkgs int
}

func (i Index) String() string {
	var pretty string
	for k, v := range i.Index {
		pretty += fmt.Sprintln(k, ": ", v)
	}
	return pretty
}

func (i Index) Save() {
	file, err := os.Create(indexFile)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Serializing index of size %v to file", len(i.Index))

	enc := gob.NewEncoder(file)
	err = enc.Encode(i)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()
}

func updateIndex(term string, pack string, path string) *DocTerm {
	term = strings.TrimSpace(term)
	term = strings.ToLower(term)

	docMap, present := index.Index[term]
	if !present {
		// new DocMap
		index.Index[term] = make(DocMap)
		docMap = index.Index[term]
	}
	_, present = docMap[path]
	if !present {
		//new docTerm
		docMap[path] = &DocTerm{
			Term:      term,
            Pack:      pack,
			Path:      path,
			Functions: 0,
			Imports:   0,
			Packages:  0,
			Types:     0,
		}
	}
	return docMap[path]
}

// walkFiles starts a goroutine to walk the directory tree at root and send the
// path of each regular file on the string channel.  It sends the result of the
// walk on the error channel.  If done is closed, walkFiles abandons its work.
func walkDirs(done <-chan struct{}, root string) (<-chan string, <-chan error) {
	dirs := make(chan string)
	errc := make(chan error, 1)
	go func() { // HL
		// Close the paths channel after Walk returns.
		defer close(dirs) // HL
		// No select needed for this send, since errc is buffered.
		errc <- filepath.Walk(root, func(dir string, info os.FileInfo, err error) error { // HL
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return nil
			}
			select {
			case dirs <- dir:
			case <-done:
				return errors.New("walk canceled")
			}
			return nil
		})
	}()
	return dirs, errc
}

// A result is the product of parsing a package into an AST
type result struct {
	pkgs   map[string]*ast.Package
	prefix string
	err    error
}

// digester reads path names from paths and sends digests of the corresponding
// files on c until either paths or done is closed.
func dirParser(done <-chan struct{}, dirs <-chan string, c chan<- result) {
	for dir := range dirs {
		fset := token.NewFileSet()
		//fmt.Println("Parseing: ", dir)
		pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)

		select {
		case c <- result{pkgs, dir, err}:
		case <-done:
			return
		}
	}
}

// MD5All reads all the files in the file tree rooted at root and returns a map
// from file path to the MD5 sum of the file's contents.  If the directory walk
// fails or any read operation fails, MD5All returns an error.  In that case,
// MD5All does not wait for inflight read operations to complete.
func indexer(root string) error {
	// MD5All closes the done channel when it returns; it may do so before
	// receiving all the values from c and errc.
	done := make(chan struct{})
	defer close(done)

	dirs, errc := walkDirs(done, root)

	// Start a fixed number of goroutines to read and digest files.
	c := make(chan result) // HLc
	var wg sync.WaitGroup
	wg.Add(numDirParsers)
	for i := 0; i < numDirParsers; i++ {
		go func() {
			dirParser(done, dirs, c) // HLc
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(c) // HLc
	}()

	for r := range c {
		if r.err != nil {
			//log.Println("In DirParser:", r.err)
			//return
			continue
		}
        
        absPath, _ := filepath.Abs(r.prefix)
        goPath := strings.TrimPrefix(absPath, "/home/ubuntu/")
		err := indexPackages(r.pkgs, goPath)
		if err != nil {
			log.Println("In AST Parser:", err)
		}
	}
	// Check whether the Walk failed.
	if err := <-errc; err != nil { // HLerrc
		log.Println("In Walk:")
		return err
	}
	return nil
}

func tokenizeCamelCase(str string) []string {
	var words []string
	l := 0
	for s := str; s != ""; s = s[l:] {
		l = strings.IndexFunc(s[1:], unicode.IsUpper) + 1
		if l <= 0 {
			l = len(s)
		}
		words = append(words, s[:l])
	}

	return words
}

// This is a long function definitions spanning multiple
// lines and all relates to a single comment related to a single
// function
func indexPackages(pkgs map[string]*ast.Package, prefix string) error {
	for name, pkg := range pkgs {
		path := prefix
        pack := name
		//fmt.Println("Inspecting ", path)

		ast.Inspect(pkg, func(n ast.Node) bool {

			switch x := n.(type) {
			//Packages
			case *ast.Package:
				if x.Name != "" {
					//update index and docMap if necessary
					docTerm := updateIndex(x.Name, pack, path)
					//update docTerm
					docTerm.Packages += 1
				}
				break

			//Imports
			case *ast.ImportSpec:
				if x.Path.Value != "" {
					//update index and docMap if necessary
					docTerm := updateIndex(strings.Replace(x.Path.Value, "\"", "", -1), pack, path)
					//update docTerm
					docTerm.Imports += 1
				}
				break

			//Functions
			case *ast.FuncDecl:
				if x.Name.Name != "" {
					//Name tokenize function
					for _, n := range tokenizeCamelCase(x.Name.Name) {
						//update index and docMap if necessary
						docTerm := updateIndex(n, pack, path)
						//update docTerm
						docTerm.Functions += 1
					}

					//Add comments to index
					if x.Doc != nil && *commentParse {
						comment := ""
						for _, c := range x.Doc.List {
							comment += c.Text
						}

						comment = strings.Replace(comment, "//", "", -1)
						comment = strings.ToLower(comment)

						words := strings.Fields(comment)

						for _, word := range words {
							docTerm := updateIndex(word, pack, path)
							docTerm.Functions += 1
						}
					}
				}
				break

			case *ast.TypeSpec:
				if x.Name.Name != "" {
					//Name tokenize function
					for _, n := range tokenizeCamelCase(x.Name.Name) {
						//update index and docMap if necessary
						docTerm := updateIndex(n, pack, path)
						//update docTerm
						docTerm.Types += 1
					}

					//Add comments to index
					if x.Doc != nil && *commentParse {
						comment := ""
						for _, c := range x.Doc.List {
							comment += c.Text
						}

						comment = strings.Replace(comment, "//", "", -1)
						comment = strings.ToLower(comment)

						words := strings.Fields(comment)

						for _, word := range words {
							docTerm := updateIndex(word, pack, path)
							docTerm.Types += 1
						}
					}
				}
				break
			}
			return true
		})
	}

	return nil
}
func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	if *commentParse {
		log.Println("Parsing Comments")
	}
	t0 := time.Now()

	log.Println(*inputPath)
	err := indexer(*inputPath)
	if err != nil {
		log.Println(err)
	}
	//Count the number of packages
	results := make(map[string]struct{}, 0)
	for _, docMap := range index.Index {
		for path, _ := range docMap {
			results[path] = struct{}{}
		}
	}
	index.UniquePkgs = len(results)

	t1 := time.Now()
	log.Printf("Indexed %v unique terms in %v packages in %v:", len(index.Index), len(results), t1.Sub(t0))

	//Save index to file
	t0 = time.Now()
	index.Save()
	t1 = time.Now()
	log.Printf("Wrote index file in %v", t1.Sub(t0))

}
