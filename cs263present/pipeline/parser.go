package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	_"runtime"
	"strings"
	"time"
	"unicode"
)

var (
	inputPath = flag.String("in", "", "Input file to parse")
	maxDirs   = flag.Int("max", -1, "Maximum # of files to parse")
	verbose   = flag.Int("v", 0, "Print the resulting index map")
	commentParse = flag.Bool("c", false, "Parse with comments?")
	index     = Index{Index:make(map[string]DocMap)}
)

const dbfile = "./index.gob"

type DocTerm struct {
	Title     string
	Path      string //github import path -- should work with go get
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
    Index map[string]DocMap
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
	file, err := os.Create(dbfile)
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

func updateIndex(term string, path string) *DocTerm {
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
			Title:     term,
			Path:      path,
			Functions: 0,
			Imports:   0,
			Packages:  0,
			Types:     0,
		}
	}
	return docMap[path]
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
		path := prefix + "/" + name
		fmt.Println("Inspecting ", path)

		ast.Inspect(pkg, func(n ast.Node) bool {

			switch x := n.(type) {
			//Packages
			case *ast.Package:
				if x.Name != "" {
					//update index and docMap if necessary
					docTerm := updateIndex(x.Name, path)
					//update docTerm
					docTerm.Packages += 1
				}
				break

			//Imports
			case *ast.ImportSpec:
				if x.Path.Value != "" {
					//update index and docMap if necessary
					docTerm := updateIndex(strings.Replace(x.Path.Value, "\"", "", -1), path)
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
						docTerm := updateIndex(n, path)
						//update docTerm
						docTerm.Functions += 1
					}

					//Add comments to index
					if x.Doc != nil && *commentParse{
						comment := ""
						for _, c := range x.Doc.List {
							comment += c.Text
						}

						comment = strings.Replace(comment, "//", "", -1)
						comment = strings.ToLower(comment)


						words := strings.Fields(comment)

						for _, word := range words {
							docTerm := updateIndex(word, path)
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
						docTerm := updateIndex(n, path)
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
							docTerm := updateIndex(word, path)
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

type workUnit struct {
	pkgs   map[string]*ast.Package
	prefix string
}

//We want to just visit Dirs in this case
func parse(path string, pkgc chan workUnit) error {
	count := 0
	err := filepath.Walk(path, func(fp string, fi os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if !fi.IsDir() {
			return nil
		}
		fmt.Println("Walking ", fp)
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, fp, nil, parser.ParseComments)
		//This lets us process while we wait for the file IO and parser. Should
		//might provide speedup even if we run program single-threaded (OS
		//threads)

		// returning non-nil stops filewalking
		if err != nil {
			log.Println("In walker:", err)
			return nil
		}
		pkgc <- workUnit{pkgs, fp}

		count++
		if *maxDirs != -1 && count > *maxDirs {
			return fmt.Errorf("maxdirs exceeded")
		}
		return nil
	})

	return err
}
func indexer(pkgc chan workUnit, reqChan chan chan error) {
	var pkgs workUnit

	for {
		select {
		case respChan := <-reqChan:
			log.Println("Indexer is quitting!")
			respChan <- nil
			return
		case pkgs = <-pkgc:
			fmt.Println("Recieved a pkg map on channel: doin things...")
			err := indexPackages(pkgs.pkgs, pkgs.prefix)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func main() {
	flag.Parse()
	//runtime.GOMAXPROCS(runtime.NumCPU())

	//Record start time
	startTime := time.Now()

	//request/reponse for indexer to quit in stable state
	reqChan := make(chan chan error)
	pkgc := make(chan workUnit)
	go indexer(pkgc, reqChan)
	err := parse(*inputPath, pkgc)

	//ask indexer to finish
	respChan := make(chan error)
	reqChan <- respChan
	err = <-respChan
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Indexer exits cleanly")

	//Record end time
	endTime := time.Now()

	//Output the results if flag is set
	if *verbose == 1 {
		fmt.Printf("%v", index.Index)
	}


    //Count the number of packages
    results := make(map[string]bool, 0)
    
    for _, docMap := range index.Index {
        for path, _ := range docMap {
            results[path] = true
        }
    }
    
    index.UniquePkgs = len(results)
    

    //Save index to file
	index.Save()
    
    
    fmt.Printf("\n\nIndexed %v unique terms in %v packages in %v\n\n", len(index.Index), len(results), endTime.Sub(startTime))

	if *verbose != 1 {
		fmt.Printf("Use -v 1 to print the resulting index map \n\n")
	}
}
