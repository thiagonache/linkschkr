package main

import (
	"flag"
	"fmt"
	"io"
	"links"
	"os"
)

func main() {
	flagSet := flag.NewFlagSet("flags", flag.ExitOnError)
	debug := flagSet.Bool("debug", false, "Run in debug mode")
	quite := flagSet.Bool("quite", false, "Outputs nothing but the final statistics")
	noRecursion := flagSet.Bool("no-recursion", false, "Does not run recursively")
	interval := flagSet.Int("interval", 2000, "Interval between each check in milliseconds")
	timeout := flagSet.Int("timeout", 4000, "Timeout between each check in milliseconds")
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
		links.WithTimeoutInMs(*timeout),
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
