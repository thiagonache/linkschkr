package linkschkr

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
)

type Check struct {
	alreadyChecked  map[string]bool
	Done            chan bool
	Limit, NWorkers int
	Output          io.Writer
	Recursive       bool
	Result          chan *Result
	URLs            []string
	Work            chan *Work
}

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
		var resp *http.Response
		var err error
		// Avoid to download binaries
		switch {
		case strings.HasSuffix(work.site, ".gz"):
			resp, err = http.Head(work.site)
		case strings.HasSuffix(work.site, ".bz2"):
			resp, err = http.Head(work.site)
		case strings.HasSuffix(work.site, ".msi"):
			resp, err = http.Head(work.site)
		case strings.HasSuffix(work.site, ".zip"):
			resp, err = http.Head(work.site)
		case strings.HasSuffix(work.site, ".pkg"):
			resp, err = http.Head(work.site)
		default:
			resp, err = http.Get(work.site)
		}
		if err != nil || resp.StatusCode != 200 {
			result.up = false
			work.result <- result
			continue
		}

		extraURIs := ParseHREF(resp.Body)
		result.extraURIs = extraURIs
		work.result <- result
	}
}

func ParseHREF(r io.Reader) []string {
	URIs := []string{}

	doc, err := html.Parse(r)
	if err != nil {
		log.Fatal(err)
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if strings.HasPrefix(a.Val, "/") {
						URIs = append(URIs, a.Val)
					}

				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return URIs
}

func SendWork(s string, work chan *Work, results chan *Result) {
	work <- &Work{
		site:   s,
		result: results,
	}
}

func Run(w io.Writer, nWorkers int, sites []string) {
	work := make(chan *Work)
	results := make(chan *Result)

	for n := 0; n < nWorkers; n++ {
		go Crawler(n, work)
	}
	alreadyChecked := map[string]struct{}{}
	for _, s := range sites {
		alreadyChecked[s] = struct{}{}
		go SendWork(s, work, results)
	}
	for {
		v := <-results
		alreadyChecked[v.site] = struct{}{}
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
				go SendWork(s, work, results)
			}
		}
		fmt.Fprintf(w, "Site %q is %q.\n", v.site, up)
	}
}

type Option func(*Check)

func New(URLs []string, opts ...Option) *Check {
	result := make(chan *Result)
	work := make(chan *Work)
	done := make(chan bool)

	chk := &Check{
		alreadyChecked: make(map[string]bool),
		Done:           done,
		NWorkers:       3,
		Output:         os.Stdout,
		Recursive:      true,
		Result:         result,
		URLs:           URLs,
		Work:           work,
	}
	for _, o := range opts {
		o(chk)
	}
	return chk
}

func WithNumberWorkers(n int) Option {
	return func(c *Check) {
		c.NWorkers = n
	}
}

func WithRunRecursively(b bool) Option {
	return func(c *Check) {
		c.Recursive = b
	}
}

func WithOutput(w io.Writer) Option {
	return func(c *Check) {
		c.Output = w
	}
}

func (chk *Check) Run() error {
	for x := 0; x < chk.NWorkers; x++ {
		go chk.Fetcher(chk.Result)
	}
	for _, url := range chk.URLs {
		go SendWork(url, chk.Work, chk.Result)
	}
	for {
		select {
		case <-chk.Done:
			return nil
		case v := <-chk.Result:
			chk.alreadyChecked[v.site] = true
			up := "up"
			if !v.up {
				up = "down"
			}
			fmt.Fprintf(chk.Output, "Site %q is %q.\n", v.site, up)
			//fmt.Printf("Site %q is %q.\n", v.site, up)
			if !chk.Recursive {
				return nil
			}
			if len(v.extraURIs) == 0 {
				return nil
			}

			for _, u := range v.extraURIs {
				url := strings.Split(v.site, "/")
				s := fmt.Sprintf("%s//%s%s", url[0], url[2], u)
				if !chk.alreadyChecked[s] {
					chk.alreadyChecked[s] = true
					go SendWork(s, chk.Work, chk.Result)
				}
			}

		}
	}
}

func (chk *Check) Fetcher(c chan<- *Result) {
	for {
		select {
		case v := <-chk.Work:
			c <- &Result{
				site:      v.site,
				up:        true,
				extraURIs: []string{"/doc"},
				workerID:  0,
			}
		case <-chk.Done:
			return
		}
	}
}
