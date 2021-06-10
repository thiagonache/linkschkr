package main

import (
	"flag"
	"fmt"
	"io"
	"links"
	"os"
	"time"
)

func logger(component, msg string) {
	fmt.Fprintf(os.Stdout, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func main() {
	flagSet := flag.NewFlagSet("flags", flag.ExitOnError)
	debug := flagSet.Bool("debug", false, "Run in debug mode")
	quite := flagSet.Bool("quite", false, "Outputs nothing but the final statistics")
	noRecursion := flagSet.Bool("no-recursion", false, "Does not run recursively")
	interval := flagSet.Int("interval", 2000, "Interval between each check in milliseconds")
	flagSet.Parse(os.Args[1:])
	if flagSet.NArg() < 1 {
		fmt.Println("Please, specify the sites as arguments")
		os.Exit(1)
	}
	sites := flagSet.Args()
	writer := io.Discard
	if *debug {
		writer = os.Stderr
	}
	if *quite {
		writer = io.Discard
	}
	failures, err := links.Check(sites,
		links.WithDebug(writer),
		links.WithQuite(*quite),
		links.WithNoRecursion(*noRecursion),
		links.WithIntervalInMs(*interval),
	)
	if err != nil {
		logger("main", err.Error())
		os.Exit(1)
	}
	logger("main", "Failures:")
	for _, fail := range failures {
		logger("main", fmt.Sprintf("URL: %q Response: %d State: %q Err: %v Refer: %q\n", fail.URL, fail.ResponseCode, fail.State, fail.Error, fail.Refer))
	}
}
