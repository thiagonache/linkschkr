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
	responseCode int
	state        string
	url          string
}

type option func(*links)

type links struct {
	debug         io.Writer
	fails         chan *Result
	httpClient    http.Client
	interval      time.Duration
	quite         bool
	recursive     bool
	resultFail    []*Result
	resultSuccess []*Result
	stdout        io.Writer
	successes     chan *Result
	waitGroup     sync.WaitGroup
}

func NewResult(url string, refer string) *Result {
	return &Result{url: url, refer: refer}
}

func (n *Result) SetStatus(state string, respCode int) {
	n.state, n.responseCode = state, respCode
}

func (n *Result) SetError(err error) {
	n.err = err
}

func logger(w io.Writer, component string, msg string) {
	fmt.Fprintf(w, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func (l *links) doRequest(method, url string, client *http.Client) (*http.Response, error) {
	client.Timeout = l.interval + ((l.interval * 10) / 100)
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
	result := &Result{url: wrk.url, refer: wrk.refer}
	resp, err := l.doRequest("HEAD", wrk.url, client)
	if err != nil {
		result.SetStatus("unknown", http.StatusInternalServerError)
		result.SetError(err)
		l.fails <- result
		return
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		result.SetStatus("down", resp.StatusCode)
		l.fails <- result
		return
	}
	ct := resp.Header.Get("Content-Type")
	logger(l.debug, "Fetcher", fmt.Sprintf("Content type %s", ct))
	if !strings.HasPrefix(ct, "text/html") {
		result.SetStatus("up", resp.StatusCode)
		l.successes <- result
		return
	}
	logger(l.debug, "Fetcher", "Run GET method")
	resp, err = l.doRequest("GET", wrk.url, client)
	if err != nil {
		result.SetStatus("unknown", http.StatusInternalServerError)
		result.SetError(err)
		l.fails <- result
		return
	}
	logger(l.debug, "Fetcher", fmt.Sprintf("response code %d", resp.StatusCode))
	logger(l.debug, "Fetcher", "done")
	if resp.StatusCode != http.StatusOK {
		result.SetStatus("down", resp.StatusCode)
		l.fails <- result
		return
	}
	if l.recursive {
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
	result.SetStatus("up", resp.StatusCode)
	l.successes <- result
}

func (l *links) parseBody(r io.Reader, site string) ([]string, error) {
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
			logger(l.debug, "ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "https://"):
			logger(l.debug, "ParseBody", "not implemented yet")
		}
	}
	return extraURLs, nil
}

func (l *links) readResults() {
	for {
		select {
		case s := <-l.successes:
			logger(l.debug, "ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", s.url, s.state, s.responseCode, s.refer, s.err))
			l.resultSuccess = append(l.resultSuccess, s)
			l.waitGroup.Done()
		case f := <-l.fails:
			logger(l.debug, "ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", f.url, f.state, f.responseCode, f.refer, f.err))
			l.resultFail = append(l.resultFail, f)
			l.waitGroup.Done()
		}
	}
}

func Check(site string, opts ...option) []*Result {
	l := &links{
		debug:         io.Discard,
		interval:      1 * time.Second,
		resultFail:    []*Result{},
		httpClient:    http.Client{},
		recursive:     true,
		resultSuccess: []*Result{},
		successes:     make(chan *Result),
		fails:         make(chan *Result),
		stdout:        os.Stdout,
	}
	for _, o := range opts {
		o(l)
	}

	if l.quite {
		l.debug = io.Discard
		l.stdout = io.Discard
	}
	chked := &checked{
		items: map[string]struct{}{},
	}
	go l.readResults()
	limiter := time.NewTicker(l.interval)
	l.waitGroup.Add(1)
	chked.existOrAdd(site)
	go l.fetch(work{url: site}, chked, limiter)
	l.waitGroup.Wait()

	return l.resultFail
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
