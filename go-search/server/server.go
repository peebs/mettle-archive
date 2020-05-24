// This package implements a simple HTTP server providing a REST API to a task handler.
//
// It provides four methods:
//
// 	GET    /search/        Start query and return results
// Every method below gives more information about every API call, its parameters, and its results.

package server

import (
	"encoding/json"
	"log"
	"net/http"

	"go-search/search"

	"github.com/gorilla/mux"
)

const PathPrefix = "/search/"

func RegisterHandlers() {
	r := mux.NewRouter()
	r.HandleFunc(PathPrefix, errorHandler(NewSearch)).Methods("POST")
	http.Handle(PathPrefix, r)
}

// badRequest is handled by setting the status code in the reply to StatusBadRequest.
type badRequest  struct { error }

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
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			log.Println(err)
			http.Error(w, "oops", http.StatusInternalServerError)
		}
	}
}

// NewSearch handles GET requests on /search.
// The request body must contain a JSON object with a Title field.
// The status code of the response is used to indicate any error.
//
// Examples:
//
//   req: POST /search/ {"Query": ""}
//   res: 200 {"Results": [
//          {"Title": "Example Code Package", "Path": "example.com"},
//          {"Title": "Example Code Package", "Path": "example.com"},
//        ]}
func NewSearch(w http.ResponseWriter, r *http.Request) error {
	req := struct{ Query string }{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return badRequest{err}
	}
	results, err := search.Run(req.Query)
	if err != nil {
		return badRequest{err}
	}

    //Results come back as pointers to the structs
    //  Need them as actual values for JSON

    var res = make([]search.Result, len(results))
    
    i := 0
    for _, v := range results {
        res[i] = *v
        i++
    }

	ret := struct{ Results []search.Result }{res}
	return json.NewEncoder(w).Encode(ret)
}
