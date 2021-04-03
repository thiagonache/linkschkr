package linkschkr

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"golang.org/x/net/html"
)

type Work struct {
	site   string
	result chan *Result
}

type Result struct {
	site      string
	up        bool
	extraURLs []string
	workerID  int
}

func Crawler(ID int, c <-chan *Work) {
	for {
		work := <-c
		result := &Result{
			site:     work.site,
			up:       true,
			workerID: ID,
		}

		resp, err := http.Get(work.site)
		if err != nil || resp.StatusCode != 200 {
			result.up = false
			work.result <- result
			continue
		}

		extraSites := ParseHREF(resp.Body)
		result.extraURLs = extraSites
		work.result <- result
	}
}

func ParseHREF(r io.Reader) []string {
	links := []string{}

	doc, err := html.Parse(r)
	if err != nil {
		log.Fatal(err)
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					links = append(links, a.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return links
}

func Run(sites []string) {
	nWorkers := 2
	work := make(chan *Work)
	results := make(chan *Result)

	for n := 0; n < nWorkers; n++ {
		go Crawler(n, work)
	}

	for _, s := range sites {
		go func(s string) {
			work <- &Work{
				site:   s,
				result: results,
			}
		}(s)
	}

	for {
		v := <-results
		up := "up"
		if !v.up {
			up = "down"
		}
		for _, s := range v.extraURLs {
			go func(s string) {
				work <- &Work{
					site:   s,
					result: results,
				}
			}(s)
		}
		fmt.Printf("Site %q is %q.\nThese links were found: %q\n", v.site, up, v.extraURLs)
	}
}
