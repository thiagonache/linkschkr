package main

import (
	"flag"
	"io"
	"links"
	"log"
	"os"
)

func main() {
	site := flag.String("site", "", "URL to check links")
	debug := flag.Bool("debug", false, "Run in debug mode")
	quite := flag.Bool("quite", false, "Outputs nothing but the final statistics")
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
	links.Run(*site,
		links.WithDebug(writer),
		links.WithQuite(*quite),
	)
}
