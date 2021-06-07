package main

import (
	"flag"
	"fmt"
	"io"
	"links"
	"log"
	"os"
)

func main() {
	// I need help to improve this later on
	site := flag.String("site", "", "URL to check links")
	debug := flag.Bool("debug", false, "Run in debug mode")
	quite := flag.Bool("quite", false, "Outputs nothing but the final statistics")
	recursive := flag.Bool("recursive", true, "Run recursively")

	flag.Parse()
	if *site == "" {
		log.Fatal("Missing -site argument")
	}
	writer := io.Discard
	if *debug {
		writer = os.Stderr
	}
	if *quite {
		writer = io.Discard
	}

	errors := links.Check(*site,
		links.WithDebug(writer),
		links.WithQuite(*quite),
		links.WithRecursive(*recursive),
	)
	for _, err := range errors {
		fmt.Println(err)
	}
}
