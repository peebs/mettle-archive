package search

import (
	"log"
	"math"
	"sort"
	"strings"
	"time"
	"flag"
)

var specific = flag.Bool("srank", false, "use specificity heuristic in ranking")

type Result struct {
	Context []DocTerm
	Rank    float64
    Pack    string
    Path    string
	Name    string
}

type Results []*Result

func (r Results) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r Results) Len() int           { return len(r) }
func (r Results) Less(i, j int) bool { return r[i].Rank < r[j].Rank }

func NewResult() *Result {
	return &Result{Context: make([]DocTerm, 0), Rank: 0, Name: ""}
}

//map pkg IDS to results
type ResultMap map[string]*Result

func Run(query string) (Results, error) {
	resultMap := rankQuery(query)
	results := sortResults(resultMap)
	if len(results) > 150 {
		return results[:150], nil
	}
	return results, nil
}

func sortResults(resultMap ResultMap) Results {
	results := make(Results, len(resultMap))

    i := 0
	for k, v := range resultMap {
		results[i] = v
		v.Path = k
		i++
	}
	sort.Sort(sort.Reverse(results))
	return results
}

func rankQuery(query string) ResultMap {
	t0 := time.Now()
	terms := strings.Fields(strings.ToLower(query))
	results := make(ResultMap)

	// for each term in query, get its TD-IDF, place that value in Result
	for _, t := range terms {
		docMap, ok := index.Index[t]
		if !ok {
			continue
		}
		mapLength := len(docMap)
		for _, docTerm := range docMap {
			result, ok := results[docTerm.Path]
			if !ok {
				result = NewResult()
                result.Pack = docTerm.Pack
				results[docTerm.Path] = result
			}
			result.Rank += score(docTerm, mapLength)
			result.Context = append(result.Context, *docTerm)

            if result.Name == "" {
                result.Name = t
            } else {
                result.Name += ", " + t
            }
		}
	}

	t1 := time.Now()
	log.Println("Ranking complete!")
	log.Println("Took ", t1.Sub(t0))

	return results
}

func score(docTerm *DocTerm, mapLength int) float64 {
	importmult := 1.0
	if *specific {
		docTerm.Functions *= 4
		docTerm.Types *= 2
		docTerm.Packages *= 1
		importmult = 0.5
	}
	freq := float64(docTerm.Functions)
	freq += float64(docTerm.Imports) * importmult
	freq += float64(docTerm.Packages)
	freq += float64(docTerm.Types)

	tfidf := freq * math.Log(float64(index.UniquePkgs)/float64(mapLength))
	return tfidf

}
