// +build OMIT

package main
 
import "fmt"

// START OMIT
type Inner struct {
	Data string
}
func (i Inner) String() string {return i.Data}

type Outer struct {
	Inner
}

func main() {
	var o Outer
	o.Data = "This works!"
	fmt.Println(o.String())
}
// STOP OMIT
