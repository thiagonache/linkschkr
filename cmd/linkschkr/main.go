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
	interval := flag.Int("interval", 2000, "Interval between each check in milliseconds")
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
	failures, err := links.Check(*site,
		links.WithDebug(writer),
		links.WithQuite(*quite),
		links.WithNoRecursion(*noRecursion),
		links.WithIntervalInMs(*interval),
	)
	if err != nil {
		links.Logger(os.Stderr, "main", err.Error())
		os.Exit(1)
	}
	links.Logger(os.Stdout, "main", "Failures:")
	for _, fail := range failures {
		links.Logger(os.Stdout, "main", fmt.Sprintf("URL: %q Response: %d State: %q Err: %v Refer: %q\n", fail.URL, fail.ResponseCode, fail.State, fail.Error, fail.Refer))
	}
}
