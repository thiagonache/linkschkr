package links

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Checker struct {
	alreadyChecked map[string]bool
	BrokenLinks    []Result
	Done           chan bool
	HTTPClient     http.Client
	Limit          time.Duration
	NWorkers       int
	Output         io.Writer
	Recursive      bool
	Result         chan *Result
	SuccessLinks   []Result
	URL            string
	Work           chan *Work
}

type Work struct {
	result chan *Result
	site   string
}

type Result struct {
	Err        error
	ExtraSites []string
	StatusCode int
	URL        string
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

type Option func(*Checker)

func NewChecker(URL string, opts ...Option) *Checker {
	result := make(chan *Result)
	work := make(chan *Work)

	chk := &Checker{
		alreadyChecked: make(map[string]bool),
		BrokenLinks:    []Result{},
		Limit:          500,
		Output:         os.Stdout,
		Recursive:      true,
		Result:         result,
		SuccessLinks:   []Result{},
		URL:            URL,
		Work:           work,
	}
	for _, o := range opts {
		o(chk)
	}
	return chk
}

func WithWorkers(n int) Option {
	return func(c *Checker) {
		c.NWorkers = n
	}
}

func WithRunRecursively(b bool) Option {
	return func(c *Checker) {
		c.Recursive = b
	}
}

func WithOutput(w io.Writer) Option {
	return func(c *Checker) {
		c.Output = w
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Checker) {
		c.HTTPClient = client
	}
}

func (c *Checker) Run() error {
	tasks := 0
	for x := 0; x < c.NWorkers; x++ {
		go c.Fetcher(c.Result)
	}
	tasks++
	go SendWork(c.URL, c.Work, c.Result)
	for v := range c.Result {
		tasks--
		c.alreadyChecked[v.URL] = true
		if v.Err != nil || v.StatusCode != http.StatusOK {
			c.BrokenLinks = append(c.BrokenLinks, *v)
		}
		if !c.Recursive {
			return nil
		}
		for _, s := range v.ExtraSites {
			if !c.alreadyChecked[s] {
				c.alreadyChecked[s] = true
				tasks++
				go SendWork(s, c.Work, c.Result)
			}
		}
		if tasks == 0 {
			return nil
		}
	}
	return nil
}

func (c *Checker) Fetcher(results chan<- *Result) {
	for v := range c.Work {
		result := &Result{
			URL: v.site,
		}
		resp, err := c.HTTPClient.Head(v.site)
		if err != nil {
			result.Err = err
			results <- result
			continue
		}
		result.StatusCode = resp.StatusCode
		if resp.StatusCode != http.StatusOK {
			results <- result
			continue
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			results <- result
			break
		}
		resp, err = c.HTTPClient.Get(v.site)
		if err != nil {
			result.Err = err
			results <- result
			continue
		}
		result.StatusCode = resp.StatusCode
		if resp.StatusCode != http.StatusOK {
			results <- result
			continue
		}
		extraURIs := ParseHREF(resp.Body)
		for _, uri := range extraURIs {
			url := strings.Split(v.site, "/")
			s := fmt.Sprintf("%s//%s%s", url[0], url[2], uri)
			if !c.alreadyChecked[s] {
				result.ExtraSites = append(result.ExtraSites, s)
			}
		}

		results <- result
	}
}
