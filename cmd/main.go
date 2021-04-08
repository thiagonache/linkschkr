package main

import (
	"links"
	"log"
)

func main() {
	lk := links.NewChecker([]string{"https://golang.org"})
	if err := lk.Run(); err != nil {
		log.Fatal(err)
	}
}
