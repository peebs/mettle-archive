package main

import "fmt"

func main() {
	// START OMIT
	list := []interface{}{0, "cat", "bat", true, 1239}
	for _, item := range list {
		fmt.Println(item)
	}
	for _, item := range list {
		switch v := item.(type) {
		case int:
			fmt.Println(v, "(int)")
		case string:
			fmt.Println(v, "(string)")
		case bool:
			fmt.Println(v, "(bool)")
		}
	}
	// STOP OMIT
}

