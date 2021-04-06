package main

import (
	"linkschkr"
	"log"
)

func main() {
	lk := linkschkr.New([]string{"https://golang.org"})
	if err := lk.Run(); err != nil {
		log.Fatal(err)
	}
}
