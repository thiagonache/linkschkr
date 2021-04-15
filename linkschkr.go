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

func (c *Checker) SendWork(s string) {
	go c.Fetcher(c.Result)
	c.Work <- &Work{
		site:   s,
		result: c.Result,
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

func WithHTTPClient(client http.Client) Option {
	return func(c *Checker) {
		c.HTTPClient = client
	}
}

func WithRateLimit(ms int) Option {
	return func(c *Checker) {
		c.Limit = time.Duration(ms)
	}
}

func (c *Checker) Run() ([]Result, []Result, error) {
	total := 0
	tasks := 0
	errors := 0

	limiter := time.NewTicker(c.Limit * time.Millisecond)
	tasks++
	go c.SendWork(c.URL)
	for v := range c.Result {
		tasks--
		total++
		c.alreadyChecked[v.URL] = true
		if v.Err != nil || v.StatusCode != http.StatusOK {
			errors++
			c.BrokenLinks = append(c.BrokenLinks, *v)
		} else {
			c.SuccessLinks = append(c.SuccessLinks, *v)
		}
		if !c.Recursive {
			return c.SuccessLinks, c.BrokenLinks, nil
		}
		for _, s := range v.ExtraSites {
			if !c.alreadyChecked[s] {
				c.alreadyChecked[s] = true
				tasks++
				<-limiter.C
				go c.SendWork(s)
			}
		}
		if tasks == 0 {
			return c.SuccessLinks, c.BrokenLinks, nil
		}

	}
	return c.SuccessLinks, c.BrokenLinks, nil
}

func (c *Checker) DoRequest(method, site string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest(method, site, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "Linkschkr 0.0.1 Beta")
	req.Header.Set("accept", "*/*")
	resp, err := client.Do(req)

	return resp, err
}
func (c *Checker) Fetcher(results chan<- *Result) {
	client := c.HTTPClient
	for v := range c.Work {
		result := &Result{
			URL: v.site,
		}
		resp, err := c.DoRequest("HEAD", v.site, &client)
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
		resp, err = c.DoRequest("GET", v.site, &client)
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
			urlFull := fmt.Sprintf("%s//%s%s", url[0], url[2], uri)
			urlNoQueryString := strings.Split(urlFull, "?")[0]
			result.ExtraSites = append(result.ExtraSites, urlNoQueryString)
		}

		results <- result
	}
}
