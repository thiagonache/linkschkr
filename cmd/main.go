package main

import (
	"fmt"
	"links"
	"log"
)

func main() {
	lk := links.NewChecker("https://bitfieldconsulting.com/")
	success, failures, err := lk.Run()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("SUCCESS")
	for _, s := range success {
		fmt.Printf("Link: %s StatusCode: %d Error: %v\n", s.URL, s.StatusCode, s.Err)
	}
	fmt.Println()
	fmt.Println("FAILURES")
	for _, f := range failures {
		fmt.Printf("Link: %s StatusCode: %d Error: %v\n", f.URL, f.StatusCode, f.Err)
	}
}
