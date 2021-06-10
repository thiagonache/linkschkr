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
	Error        error
	Refer        string
	ResponseCode int
	State        string
	URL          string
}

type option func(*checker)

type checker struct {
	debug      io.Writer
	domain     string
	httpClient http.Client
	interval   time.Duration
	quiet      bool
	recursive  bool
	responses  []Result
	results    chan Result
	scheme     string
	stats      stats
	stdout     io.Writer
	wg         sync.WaitGroup
}

func Check(sites []string, opts ...option) ([]Result, error) {
	l := &checker{
		debug:      io.Discard,
		httpClient: http.Client{},
		interval:   2000 * time.Millisecond,
		recursive:  true,
		responses:  []Result{},
		results:    make(chan Result),
		stdout:     os.Stdout,
	}
	for _, o := range opts {
		o(l)
	}
	if l.quiet {
		l.debug = io.Discard
		l.stdout = io.Discard
	}
	chked := &checked{
		items: map[string]bool{},
	}
	go l.readResults()
	limiter := time.NewTicker(l.interval)
	for _, site := range sites {
		url, err := url.Parse(site)
		if err != nil {
			l.Log("Checker", err.Error())
			continue
		}
		l.scheme, l.domain = url.Scheme, url.Host
		l.wg.Add(1)
		chked.isBeingChecked(site)
		go l.fetch(work{site: site}, chked, limiter)
		l.wg.Wait()
	}

	l.Log("Checker", fmt.Sprintf("total checks performed is %d", l.stats.total))
	return l.failures(), nil
}

func (l *checker) Debug(component string, msg string) {
	fmt.Fprintf(l.debug, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func (l *checker) Log(component string, msg string) {
	fmt.Fprintf(l.stdout, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func (l *checker) doRequest(method, site string) (*http.Response, error) {
	req, err := http.NewRequest(method, site, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "Linkschkr 0.0.1 Beta")
	req.Header.Set("accept", "*/*")
	resp, err := l.httpClient.Do(req)

	return resp, err
}

func (l *checker) fetch(wrk work, c *checked, limiter *time.Ticker) {
	l.Debug("Fetcher", fmt.Sprintf("checking site %s", wrk.site))
	result := Result{URL: wrk.site, Refer: wrk.refer}
	resp, err := l.doRequest(http.MethodHead, wrk.site)
	if err != nil {
		result.Error = err
		l.results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	if broken(resp.StatusCode) {
		l.results <- result
		return
	}
	if nonHTML(resp.Header) {
		l.results <- result
		return
	}
	l.Debug("Fetcher", "Run GET method")
	resp, err = l.doRequest("GET", wrk.site)
	if err != nil {
		result.Error = err
		l.results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	l.Debug("Fetcher", fmt.Sprintf("response code %d", resp.StatusCode))
	if resp.StatusCode != http.StatusOK {
		l.results <- result
		return
	}
	u, err := url.Parse(wrk.site)
	if err != nil {
		result.Error = err
		l.results <- result
		return
	}
	if l.recursive && u.Host == l.domain {
		extraSites, err := l.parseBody(resp.Body, wrk.site)
		if err != nil {
			l.Log("Fetcher", "error looking for extra sites")
		}
		for _, s := range extraSites {
			if c.isBeingChecked(s) {
				continue
			}
			l.wg.Add(1)
			<-limiter.C
			go l.fetch(work{site: s, refer: wrk.site}, c, limiter)
		}
	}
	result.State = "up"
	l.results <- result
	l.Debug("Fetcher", "done")
}

func (l *checker) parseBody(r io.Reader, site string) ([]string, error) {
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
			l.Debug("ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "/"):
			baseURL := fmt.Sprintf("%s://%s", l.scheme, l.domain)
			extraURLs = append(extraURLs, fmt.Sprintf("%s%s", baseURL, href))
		case strings.HasPrefix(href, "http://"):
			extraURLs = append(extraURLs, href)
		case strings.HasPrefix(href, "https://"):
			extraURLs = append(extraURLs, href)
		}
	}
	return extraURLs, nil
}

func (l *checker) failures() []Result {
	resp := []Result{}
	for _, r := range l.responses {
		if r.ResponseCode != http.StatusOK {
			resp = append(resp, r)
		}
	}
	return resp
}

func (l *checker) readResults() {
	for r := range l.results {
		l.stats.total += 1
		l.stats.successes += 1
		r.State = "up"
		if r.ResponseCode != http.StatusOK {
			r.State = "down"
			l.stats.successes -= 1
			l.stats.failures += 1
		}
		l.Debug("ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", r.URL, r.State, r.ResponseCode, r.Refer, r.Error))
		l.responses = append(l.responses, r)
		l.wg.Done()
	}
}

func WithHTTPClient(client *http.Client) option {
	return func(l *checker) { l.httpClient = *client }
}

func WithNoRecursion(b bool) option {
	return func(l *checker) { l.recursive = !b }
}

func WithStdout(w io.Writer) option {
	return func(l *checker) { l.stdout = w }
}

func WithDebug(w io.Writer) option {
	return func(l *checker) { l.debug = w }
}

func WithQuite(quite bool) option {
	return func(l *checker) { l.quiet = quite }
}

func WithIntervalInMs(n int) option {
	return func(l *checker) { l.interval = time.Duration(n) * time.Millisecond }
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