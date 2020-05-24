// +build OMIT

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

var _ = io.Copy

func main() {
	// START OMIT
	b := new(bytes.Buffer)
	var w io.Writer
	w = b
	fmt.Fprintf(w, "hello, %s\n", "world")
	os.Stdout.Write(b.Bytes())
	// STOP OMIT
}
