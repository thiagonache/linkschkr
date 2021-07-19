package links

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/htmlquery"
)

type work struct {
	refer string
	site  string
}

type stats struct {
	failures, successes, total int
}

type checked struct {
	mu    sync.Mutex
	items map[string]bool
}

func (c *checked) isBeingChecked(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items[key] {
		return true
	}
	c.items[key] = true
	return false
}

type Result struct {
	Error        error  `json:"error"`
	Refer        string `json:"refer"`
	ResponseCode int    `json:"responseCode"`
	State        string `json:"state"`
	URL          string `json:"url"`
}

type Option func(*checker)

type checker struct {
	debug      io.Writer
	domain     string
	httpClient http.Client
	interval   time.Duration
	quiet      bool
	recursive  bool
	responses  []Result
	scheme     string
	stats      stats
	stdout     io.Writer
	wg         sync.WaitGroup
}

func Check(sites []string, opts ...Option) ([]Result, error) {
	c := &checker{
		debug:      io.Discard,
		httpClient: http.Client{},
		interval:   2000 * time.Millisecond,
		recursive:  true,
		responses:  []Result{},
		stdout:     os.Stdout,
	}
	for _, o := range opts {
		o(c)
	}
	if c.quiet {
		c.debug = io.Discard
		c.stdout = io.Discard
	}
	chked := &checked{
		items: map[string]bool{},
	}
	results := make(chan Result)
	go c.readResults(results)
	limiter := time.NewTicker(c.interval)
	for _, site := range sites {
		url, err := url.Parse(site)
		if err != nil {
			return nil, err
		}
		if url.Scheme == "" || url.Host == "" {
			return nil, fmt.Errorf("invalid URL %q", url)
		}
		c.scheme, c.domain = url.Scheme, url.Host
		c.wg.Add(1)
		chked.isBeingChecked(site)
		go c.fetch(work{site: site}, chked, limiter, results)
		c.wg.Wait()
	}

	c.Log("Checker", fmt.Sprintf("total checks performed is %d", c.stats.total))
	return c.failures(), nil
}

func (c *checker) Debug(component string, msg string) {
	fmt.Fprintf(c.debug, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func (c *checker) Log(component string, msg string) {
	fmt.Fprintf(c.stdout, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func (c *checker) doRequest(method, site string) (*http.Response, error) {
	req, err := http.NewRequest(method, site, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "Linkschkr 0.0.1 Beta")
	req.Header.Set("accept", "*/*")
	resp, err := c.httpClient.Do(req)

	return resp, err
}

func (c *checker) fetch(wrk work, chked *checked, limiter *time.Ticker, results chan<- Result) {
	c.Debug("Fetcher", fmt.Sprintf("checking site %s", wrk.site))
	result := Result{URL: wrk.site, Refer: wrk.refer}
	resp, err := c.doRequest(http.MethodHead, wrk.site)
	if err != nil {
		result.Error = err
		results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	if broken(resp.StatusCode) {
		results <- result
		return
	}
	if nonHTML(resp.Header) {
		results <- result
		return
	}
	c.Debug("Fetcher", "Run GET method")
	resp, err = c.doRequest("GET", wrk.site)
	if err != nil {
		result.Error = err
		results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	c.Debug("Fetcher", fmt.Sprintf("response code %d", resp.StatusCode))
	if resp.StatusCode != http.StatusOK {
		results <- result
		return
	}
	u, err := url.Parse(wrk.site)
	if err != nil {
		result.Error = err
		results <- result
		return
	}
	if c.recursive && u.Host == c.domain {
		extraSites, err := c.parseBody(resp.Body, wrk.site)
		if err != nil {
			c.Log("Fetcher", "error looking for extra sites")
		}
		for _, s := range extraSites {
			if chked.isBeingChecked(s) {
				continue
			}
			c.wg.Add(1)
			<-limiter.C
			go c.fetch(work{site: s, refer: wrk.site}, chked, limiter, results)
		}
	}
	results <- result
	c.Debug("Fetcher", "done")
}

func (c *checker) parseBody(r io.Reader, site string) ([]string, error) {
	extraURLs := []string{}
	doc, err := htmlquery.Parse(r)
	if err != nil {
		return nil, err
	}
	list := htmlquery.Find(doc, "//a/@href")
	for _, n := range list {
		href := htmlquery.SelectAttr(n, "href")
		switch {
		case strings.HasPrefix(href, "//"):
			c.Debug("ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "/"):
			baseURL := fmt.Sprintf("%s://%s", c.scheme, c.domain)
			extraURLs = append(extraURLs, fmt.Sprintf("%s%s", baseURL, href))
		case strings.HasPrefix(href, "http://"):
			extraURLs = append(extraURLs, href)
		case strings.HasPrefix(href, "https://"):
			extraURLs = append(extraURLs, href)
		}
	}
	return extraURLs, nil
}

func (c *checker) failures() []Result {
	resp := []Result{}
	for _, r := range c.responses {
		if r.ResponseCode != http.StatusOK {
			resp = append(resp, r)
		}
	}
	return resp
}

func (c *checker) readResults(results <-chan Result) {
	for r := range results {
		c.stats.total += 1
		c.stats.successes += 1
		r.State = "up"
		if r.ResponseCode != http.StatusOK {
			r.State = "down"
			c.stats.successes -= 1
			c.stats.failures += 1
		}
		c.Debug("ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", r.URL, r.State, r.ResponseCode, r.Refer, r.Error))
		c.responses = append(c.responses, r)
		c.wg.Done()
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *checker) { c.httpClient = *client }
}

func WithNoRecursion(b bool) Option {
	return func(c *checker) { c.recursive = !b }
}

func WithStdout(w io.Writer) Option {
	return func(c *checker) { c.stdout = w }
}

func WithDebug(w io.Writer) Option {
	return func(c *checker) { c.debug = w }
}

func WithQuite(quite bool) Option {
	return func(c *checker) { c.quiet = quite }
}

func WithIntervalInMs(n int) Option {
	return func(c *checker) { c.interval = time.Duration(n) * time.Millisecond }
}

func broken(s int) bool {
	switch s {
	case http.StatusOK:
		return false
	case http.StatusMethodNotAllowed:
		return false
	case http.StatusForbidden:
		return false
	default:
		return true
	}
}

func nonHTML(header http.Header) bool {
	ct := header.Get("Content-Type")
	return !strings.HasPrefix(ct, "text/html")
}
