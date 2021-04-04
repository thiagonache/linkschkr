package linkschkr

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

type Work struct {
	site   string
	result chan *Result
}

type Result struct {
	site      string
	up        bool
	extraURIs []string
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
		result.extraURIs = extraSites
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
					if a.Val == "/" {
						continue
					}
					if strings.HasPrefix(a.Val, "/") {
						links = append(links, a.Val)
					}

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
	alreadyChecked := map[string]struct{}{}
	for _, s := range sites {
		alreadyChecked[s] = struct{}{}
		go func(s string) {
			work <- &Work{
				site:   s,
				result: results,
			}
		}(s)
	}
	//count := 0
	for {
		v := <-results
		alreadyChecked[v.site] = struct{}{}
		// count++
		// if count > 4 {
		// 	break
		// }
		up := "up"
		if !v.up {
			up = "down"
		}
		for _, u := range v.extraURIs {
			url := strings.Split(v.site, "/")
			s := fmt.Sprintf("%s//%s%s", url[0], url[2], u)
			_, ok := alreadyChecked[s]
			if !ok {
				alreadyChecked[s] = struct{}{}
				go func(s string) {
					work <- &Work{
						site:   s,
						result: results,
					}
				}(s)
			}
		}
		fmt.Printf("Site %q is %q.\nThese links were found: %q\n", v.site, up, v.extraURLs)
	}
}
