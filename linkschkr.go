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

type Work struct {
	refer string
	site  string
}

type stats struct {
	total int
}
type Checked struct {
	mu    sync.Mutex
	Items map[string]struct{}
}

func (c *Checked) existOrAdd(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.Items[key]
	if !ok {
		c.Items[key] = struct{}{}
		return false
	}
	return true
}

type Result struct {
	Error        error
	Refer        string
	ResponseCode int
	State        string
	URL          string
}

type option func(*links)

type links struct {
	debug      io.Writer
	domain     string
	httpClient http.Client
	interval   time.Duration
	quite      bool
	recursive  bool
	responses  []*Result
	results    chan *Result
	scheme     string
	stats      stats
	stdout     io.Writer
	wg         sync.WaitGroup
}

func Logger(w io.Writer, component string, msg string) {
	fmt.Fprintf(w, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func Check(site string, opts ...option) ([]*Result, error) {
	l := &links{
		debug:      io.Discard,
		httpClient: http.Client{},
		interval:   1 * time.Second,
		recursive:  true,
		responses:  []*Result{},
		results:    make(chan *Result),
		stdout:     os.Stdout,
	}
	for _, o := range opts {
		o(l)
	}
	url, err := url.Parse(site)
	if err != nil {
		return nil, err
	}
	l.scheme, l.domain = url.Scheme, url.Host
	if l.quite {
		l.debug = io.Discard
		l.stdout = io.Discard
	}
	checked := &Checked{
		Items: map[string]struct{}{},
	}
	go l.readResults()
	limiter := time.NewTicker(l.interval)
	l.wg.Add(1)
	checked.existOrAdd(site)
	go l.fetch(Work{site: site}, checked, limiter)
	l.wg.Wait()
	Logger(l.stdout, "Checker", fmt.Sprintf("total checks performed is %d", l.stats.total))
	return l.failures(), nil
}

func (l *links) doRequest(method, site string, client *http.Client) (*http.Response, error) {
	client.Timeout = l.interval * 2
	req, err := http.NewRequest(method, site, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "Linkschkr 0.0.1 Beta")
	req.Header.Set("accept", "*/*")
	resp, err := client.Do(req)

	return resp, err
}

func (l *links) fetch(work Work, c *Checked, limiter *time.Ticker) {
	Logger(l.debug, "Fetcher", "started")
	<-limiter.C
	client := &l.httpClient
	Logger(l.debug, "Fetcher", fmt.Sprintf("checking site %s", work.site))
	result := &Result{URL: work.site, Refer: work.refer}
	resp, err := l.doRequest("HEAD", work.site, client)
	if err != nil {
		result.Error = err
		l.results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		l.results <- result
		return
	}
	ct := resp.Header.Get("Content-Type")
	Logger(l.debug, "Fetcher", fmt.Sprintf("Content type %s", ct))
	if !strings.HasPrefix(ct, "text/html") {
		l.results <- result
		return
	}
	Logger(l.debug, "Fetcher", "Run GET method")
	resp, err = l.doRequest("GET", work.site, client)
	if err != nil {
		result.Error = err
		l.results <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	Logger(l.debug, "Fetcher", fmt.Sprintf("response code %d", resp.StatusCode))
	if resp.StatusCode != http.StatusOK {
		l.results <- result
		return
	}
	if l.recursive {
		extraSites, err := l.parseBody(resp.Body, work.site)
		if err != nil {
			Logger(l.stdout, "Fetcher", "error looking for extra sites")
		}
		for _, s := range extraSites {
			exist := c.existOrAdd(s)
			if !exist {
				l.wg.Add(1)
				go l.fetch(Work{site: s, refer: work.site}, c, limiter)
			}
		}
	}
	result.State = "up"
	l.results <- result
	Logger(l.debug, "Fetcher", "done")
}

func (l *links) parseBody(r io.Reader, site string) ([]string, error) {
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
			Logger(l.debug, "ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "/"):
			baseURL := fmt.Sprintf("%s://%s", l.scheme, l.domain)
			extraURLs = append(extraURLs, fmt.Sprintf("%s%s", baseURL, href))
		case strings.HasPrefix(href, "http://"):
			Logger(l.debug, "ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "https://"):
			Logger(l.debug, "ParseBody", "not implemented yet")
		}
	}
	return extraURLs, nil
}

func (l *links) failures() []*Result {
	resp := []*Result{}
	for _, r := range l.responses {
		if r.ResponseCode != http.StatusOK {
			resp = append(resp, r)
		}
	}
	return resp
}

func (l *links) readResults() {
	for r := range l.results {
		l.stats.total += 1
		r.State = "up"
		if r.ResponseCode != http.StatusOK {
			r.State = "down"
		}
		Logger(l.debug, "ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", r.URL, r.State, r.ResponseCode, r.Refer, r.Error))
		l.responses = append(l.responses, r)
		l.wg.Done()
	}
}

func WithHTTPClient(client *http.Client) option {
	return func(l *links) { l.httpClient = *client }
}

func WithNoRecursion(b bool) option {
	return func(l *links) { l.recursive = !b }
}

func WithStdout(w io.Writer) option {
	return func(l *links) { l.stdout = w }
}

func WithDebug(w io.Writer) option {
	return func(l *links) { l.debug = w }
}

func WithQuite(quite bool) option {
	return func(l *links) { l.quite = quite }
}

func WithIntervalInMs(n int) option {
	return func(l *links) { l.interval = time.Duration(n) * time.Millisecond }
}
