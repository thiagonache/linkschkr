package main

import (
	"links"
	"log"
)

func main() {
	log.Fatal(links.ListenAndServe(":8080"))
}
