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
	site := flag.String("site", "", "URL to check links")
	debug := flag.Bool("debug", false, "Run in debug mode")
	quite := flag.Bool("quite", false, "Outputs nothing but the final statistics")
	noRecursion := flag.Bool("no-recursion", false, "Does not run recursively")

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

	failures := links.Check(*site,
		links.WithDebug(writer),
		links.WithQuite(*quite),
		links.WithNoRecursion(*noRecursion),
	)
	fmt.Println("Failures:")
	for _, fail := range failures {
		fmt.Printf("URL: %q, State: %q, Err: %v\n", fail.URL, fail.State, fail.Error)
	}
}
