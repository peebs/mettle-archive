package main

import (
	"fmt"
	"time"
	"math/rand"
)

type one struct {
	a string
	b string
	c int
	d bool
}

type two struct {
	a string
	b int32
	d bool
	c int
}
type three struct {
	a uint64
	b int
	c string
	d int32
}
type four struct {
	a uint64
	b int
	c string
	d uint32
}
func (s *one) GetInt() int {
	return s.c
}

func (s *two) GetInt() int {
	return s.c
}
func (s *three) GetInt() int {
	return s.b
}
func (s *four) GetInt() int {
	return s.b
}
func (s *one) Append(str string) {
	s.a += str
}
func (s *two) Append(str string) {
	s.a += str
}
func (s *three) Append(str string) {
	s.c += str
}
func (s *four) Append(str string) {
	s.c += str
}
type intAppender interface {
	GetInt() int
	Append(string)
}

// START OMIT
//Takes list of intAppenders and uses interface functions to modify array
func genericAppend(s []intAppender) {
	for _,item := range s {
		item.Append("x")
		x := item.GetInt()
		x++
	}
}

//Takes underlying struct and calls methods directly
func staticAppend(s []one) {
	for _,item := range s {
		item.Append("x")
		x := item.GetInt()
		x++
	}
}
// STOP OMIT

func main () {
	dynamicSlice := []intAppender{}
	for i :=0; i<100000; i++ {
		j := rand.Intn(4)
		if j == 0 {
			dynamicSlice = append(dynamicSlice, &one{})
		} else if j == 1 {
			dynamicSlice = append(dynamicSlice, &two{})
		} else if j%2 == 2 {
			dynamicSlice = append(dynamicSlice, &three{})
		} else {
			dynamicSlice = append(dynamicSlice, &four{})
		}
	}
 	dynamicSlice2 := []intAppender{}
	for i :=0; i<100000; i++ {
			dynamicSlice2 = append(dynamicSlice2, &one{})
	} 
	staticSlice := []one{}
 	for i :=0; i<100000; i++ {
			staticSlice = append(staticSlice, one{})
	}
	t0 := time.Now()
	genericAppend(dynamicSlice)
	t1 := time.Now()
	fmt.Println("Generic Container took: ", t1.Sub(t0))

    //t0 = time.Now()
	//genericAppend(dynamicSlice2)
	//t1 = time.Now()
	//fmt.Println("Single Type Generic Container took: ", t1.Sub(t0))
     
	t0 = time.Now()
	staticAppend(staticSlice)
	t1 = time.Now()
	fmt.Println("Static Container took: ", t1.Sub(t0))
}
