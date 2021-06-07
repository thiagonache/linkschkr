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
	url   string
}

type checked struct {
	mu    sync.Mutex
	items map[string]struct{}
}

func (c *checked) existOrAdd(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[key]
	if !ok {
		c.items[key] = struct{}{}
		return false
	}
	return true
}

type Result struct {
	err          error
	refer        string
	ResponseCode int
	State        string
	URL          string
}

type option func(*links)

type links struct {
	debug         io.Writer
	host          string
	httpClient    http.Client
	interval      time.Duration
	quite         bool
	recursive     bool
	resultFail    []*Result
	results       chan *Result
	resultSuccess []*Result
	site          string
	stdout        io.Writer
	timeout       time.Duration
	waitGroup     sync.WaitGroup
}

func logger(w io.Writer, component string, msg string) {
	fmt.Fprintf(w, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func (l *links) doRequest(method, url string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "Linkschkr 0.0.1 Beta")
	req.Header.Set("accept", "*/*")
	resp, err := client.Do(req)
	return resp, err
}

func (l *links) fetch(wrk work, c *checked, limiter *time.Ticker) {
	logger(l.debug, "Fetcher", "started")
	<-limiter.C
	client := &l.httpClient
	logger(l.debug, "Fetcher", fmt.Sprintf("checking site %s", wrk.url))
	result := &Result{URL: wrk.url, refer: wrk.refer}
	resp, err := l.doRequest("HEAD", wrk.url, client)
	if err != nil {
		result.State, result.err = "unknown", err
		l.results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		result.State, result.ResponseCode = "down", resp.StatusCode
		l.results <- result
		return
	}
	ct := resp.Header.Get("Content-Type")
	logger(l.debug, "Fetcher", fmt.Sprintf("Content type %s", ct))
	if !strings.HasPrefix(ct, "text/html") {
		result.State, result.ResponseCode = "up", resp.StatusCode
		l.results <- result
		return
	}
	logger(l.debug, "Fetcher", "Run GET method")
	resp, err = l.doRequest("GET", wrk.url, client)
	if err != nil {
		result.State, result.err = "unknown", err
		l.results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	logger(l.debug, "Fetcher", fmt.Sprintf("response code %d", resp.StatusCode))
	logger(l.debug, "Fetcher", "done")
	if resp.StatusCode != http.StatusOK {
		result.State, result.ResponseCode = "down", resp.StatusCode
		l.results <- result
		return
	}
	u, _ := url.Parse(wrk.refer)
	if l.recursive && u.Host != l.host {
		extraSites, err := l.parseBody(resp.Body, wrk.url)
		if err != nil {
			logger(l.stdout, "Fetcher", "error looking for extra sites")
		}
		for _, s := range extraSites {
			exist := c.existOrAdd(s)
			if !exist {
				l.waitGroup.Add(1)
				go l.fetch(work{url: s, refer: wrk.url}, c, limiter)
			}
		}
	}
	result.State, result.ResponseCode = "up", resp.StatusCode
	l.results <- result
}

func (l *links) parseBody(r io.ReadCloser, site string) ([]string, error) {
	defer r.Close()
	extraURLs := []string{}
	doc, err := htmlquery.Parse(r)
	if err != nil {
		return nil, err
	}
	list := htmlquery.Find(doc, "//a/@href")
	site = strings.TrimSuffix(site, "/")
	for _, n := range list {
		href := htmlquery.SelectAttr(n, "href")
		switch {
		case strings.HasPrefix(href, "//"):
			logger(l.debug, "ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "/"):
			href = strings.TrimSuffix(href, "/")
			// I'm sure it is a valid URL because it was validated before I just
			// need to parse it again. This is why i'm ignoring error returned
			// from the url.Parse function
			u, _ := url.Parse(site)
			baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			extraURLs = append(extraURLs, fmt.Sprintf("%s%s", baseURL, href))
		case strings.HasPrefix(href, "http://"):
			extraURLs = append(extraURLs, href)
		case strings.HasPrefix(href, "https://"):
			extraURLs = append(extraURLs, href)
		}
	}
	return extraURLs, nil
}

func (l *links) readResults() {
	for r := range l.results {
		logger(l.debug, "ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", r.URL, r.State, r.ResponseCode, r.refer, r.err))
		if r.ResponseCode != http.StatusOK {
			l.resultFail = append(l.resultFail, r)
			l.waitGroup.Done()
			continue
		}
		l.resultSuccess = append(l.resultSuccess, r)
		l.waitGroup.Done()
	}
}

func newLinks(site string, opts ...option) (*links, error) {
	l := &links{
		debug:         io.Discard,
		httpClient:    http.Client{},
		interval:      1 * time.Second,
		recursive:     true,
		resultFail:    []*Result{},
		results:       make(chan *Result),
		resultSuccess: []*Result{},
		stdout:        os.Stdout,
		timeout:       1 * time.Second,
	}
	for _, o := range opts {
		o(l)
	}
	l.httpClient.Timeout = l.timeout
	if l.quite {
		l.debug = io.Discard
		l.stdout = io.Discard
	}
	u, err := url.Parse(site)
	if err != nil {
		return nil, err
	}
	l.site = site
	l.host = u.Host
	return l, nil
}

func Check(site string, opts ...option) ([]*Result, []*Result, error) {
	l, err := newLinks(site, opts...)
	if err != nil {
		return nil, nil, err
	}
	chked := &checked{
		items: map[string]struct{}{},
	}
	go l.readResults()
	limiter := time.NewTicker(l.interval)
	l.waitGroup.Add(1)
	chked.existOrAdd(l.site)
	go l.fetch(work{url: l.site}, chked, limiter)
	l.waitGroup.Wait()
	return l.resultFail, l.resultSuccess, nil
}

func WithHTTPClient(client *http.Client) option {
	return func(l *links) {
		l.httpClient = *client
	}
}

func WithRecursive(b bool) option {
	return func(l *links) {
		l.recursive = b
	}
}

func WithStdout(w io.Writer) option {
	return func(l *links) {
		l.stdout = w
	}
}

func WithDebug(w io.Writer) option {
	return func(l *links) {
		l.debug = w
	}
}

func WithQuite(quite bool) option {
	return func(l *links) {
		l.quite = quite
	}
}

func WithIntervalInMs(n int) option {
	return func(l *links) {
		l.interval = time.Duration(n) * time.Millisecond
	}
}

func WithTimeoutInMs(n int) option {
	return func(l *links) {
		l.timeout = time.Duration(n) * time.Millisecond
	}
}
