package main

import (
	"flag"
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
	intervalSec := flag.Int("interval", 3, "Interval that URLs should be checked in seconds")
	maxTimes := flag.Int("max", 1, "How many times URLs should be checked in interval defined")
	maxWaitSec := flag.Int("max-wait", 2, "How many seconds without no work before shutting down")

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
		links.WithRate(*intervalSec, *maxTimes, *maxWaitSec),
	)
}
