package main

import (
	"fmt"
	"os"
	"time"
)

// a little test process that slowly prints output
func main() {
	fmt.Printf("0\n")
	for i := 1; i < 5; i++ {
		time.Sleep(time.Second)
		fmt.Printf("%v\n", i)
	}
	fmt.Fprintf(os.Stderr, "this went to stderr\n")
}
