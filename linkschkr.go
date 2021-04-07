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
	site       string
	up         bool
	extraSites []string
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

type Option func(*Check)

func New(URLs []string, opts ...Option) *Check {
	result := make(chan *Result)
	work := make(chan *Work)

	chk := &Check{
		alreadyChecked: make(map[string]bool),
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
	tasks := 0
	for x := 0; x < chk.NWorkers; x++ {
		go chk.Fetcher(chk.Result)
	}
	for _, url := range chk.URLs {
		tasks++
		go SendWork(url, chk.Work, chk.Result)
	}
	for {
		select {
		case v := <-chk.Result:
			tasks--
			chk.alreadyChecked[v.site] = true
			up := "up"
			if !v.up {
				up = "down"
			}
			fmt.Fprintf(chk.Output, "Site %q is %q.\n", v.site, up)
			if !chk.Recursive {
				return nil
			}
			for _, s := range v.extraSites {
				if !chk.alreadyChecked[s] {
					chk.alreadyChecked[s] = true
					tasks++
					go SendWork(s, chk.Work, chk.Result)
				}
			}
			if tasks == 0 {
				return nil
			}
		}
	}
}

func (chk *Check) Fetcher(c chan<- *Result) {
	for {
		select {
		case v := <-chk.Work:
			result := &Result{
				site: v.site,
				up:   true,
			}
			resp, err := http.Head(v.site)
			if err != nil || (resp.StatusCode != 200 && resp.StatusCode != 405) {
				fmt.Println(resp.StatusCode, err)
				result.up = false
				c <- result
				continue
			}
			ct := resp.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "text/html") {
				break
			}
			resp, err = http.Get(v.site)
			extraURIs := ParseHREF(resp.Body)
			for _, uri := range extraURIs {
				url := strings.Split(v.site, "/")
				s := fmt.Sprintf("%s//%s%s", url[0], url[2], uri)
				if !chk.alreadyChecked[s] {
					result.extraSites = append(result.extraSites, s)
				}
			}

			c <- result
		}
	}
}
