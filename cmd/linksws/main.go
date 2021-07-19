package main

import (
	"links"
	"log"
)

func main() {
	ws := links.NewWebServer()
	log.Fatal(ws.ListenAndServe())
}
