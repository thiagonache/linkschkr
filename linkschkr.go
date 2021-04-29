package links

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/htmlquery"
)

type Checked struct {
	mu    sync.Mutex
	Items map[string]struct{}
}

func (c *Checked) ExistOrAdd(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.Items[key]
	if !ok {
		c.Items[key] = struct{}{}
		return false
	}
	return true
}

func (c *Checked) Get(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.Items[key]
	if !ok {
		return false
	}
	return ok
}

type Result struct {
	Error        error
	ResponseCode int
	State        string
	URL          string
}
type Option func(*Limiter)

type Limiter struct {
	Debug         io.Writer
	ResultFail    []*Result
	HTTPClient    http.Client
	Items         map[string]Rate
	Quite         bool
	Rate          Rate
	Recursive     bool
	ResultSuccess []*Result
	Successes     chan *Result
	Fails         chan *Result
	Stdout        io.Writer
	WaitGroup     sync.WaitGroup
}

type Rate struct {
	Count    int
	Interval time.Duration
	MaxRun   int
	MaxWait  time.Duration
	Start    time.Time
}

func (l *Limiter) DoRequest(method, site string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest(method, site, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "Linkschkr 0.0.1 Beta")
	req.Header.Set("accept", "*/*")
	resp, err := client.Do(req)

	return resp, err
}

func (l *Limiter) Fetch(site string, c *Checked) {
	fmt.Fprintf(l.Debug, "[%s] [%s] started\n", time.Now().UTC().Format(time.RFC3339), "Fetcher")
	client := &l.HTTPClient
	fmt.Fprintf(l.Stdout, "[%s] [%s] checking site %s\n", time.Now().UTC().Format(time.RFC3339), "Fetcher", site)
	result := &Result{
		URL: site,
	}
	resp, err := l.DoRequest("HEAD", site, client)
	if err != nil {
		result.State = "unkown"
		result.Error = err
		l.Fails <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		result.State = "down"
		l.Fails <- result
		return
	}
	ct := resp.Header.Get("Content-Type")
	fmt.Fprintf(l.Debug, "[%v] [%s] Content type %s\n", time.Now().UTC().Format(time.RFC3339), "Fetcher", ct)
	if !strings.HasPrefix(ct, "text/html") {
		result.State = "up"
		l.Successes <- result
		return
	}

	fmt.Fprintf(l.Debug, "[%v] [%s] Run GET method\n", time.Now().UTC().Format(time.RFC3339), "Fetcher")
	resp, err = l.DoRequest("GET", site, client)
	if err != nil {
		result.ResponseCode = resp.StatusCode
		result.State = "unkown"
		result.Error = err
		l.Fails <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	fmt.Fprintf(l.Debug, "[%s] [%s] response code %d\n", time.Now().UTC().Format(time.RFC3339), "Fetcher", resp.StatusCode)
	fmt.Fprintf(l.Debug, "[%s] [%s] done\n", time.Now().UTC().Format(time.RFC3339), "Fetcher")
	if resp.StatusCode != http.StatusOK {
		result.State = "down"
		l.Fails <- result
		return
	}

	extraSites := l.ParseHREF(resp.Body, site)
	for _, s := range extraSites {
		exist := c.ExistOrAdd(s)
		if !exist {
			l.WaitGroup.Add(1)
			go l.Fetch(s, c)
		}
	}

	result.State = "up"
	l.Successes <- result
}

func (l *Limiter) ParseHREF(r io.Reader, site string) []string {
	extraURLs := []string{}
	doc, err := htmlquery.Parse(r)
	if err != nil {
		log.Fatal(err)
	}
	list := htmlquery.Find(doc, "//a/@href")
	site = strings.TrimSuffix(site, "/")
	for _, n := range list {
		href := htmlquery.SelectAttr(n, "href")
		switch {
		case strings.HasPrefix(href, "//"):
			fmt.Fprintf(l.Debug, "[%s] [%s] not implemented yet\n", time.Now().UTC().Format(time.RFC3339), "ParseHREF")
		case strings.HasPrefix(href, "/"):
			shouldTrim := strings.HasSuffix(href, "/")
			if shouldTrim {
				href = strings.TrimSuffix(href, "/")
			}
			// I'm sure it is a valid URL, there is no reason for parse to fail.
			// This is why i'm ignoring error returned from Parse
			u, _ := url.Parse(site)
			baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			extraURLs = append(extraURLs, fmt.Sprintf("%s%s", baseURL, href))
		case strings.HasPrefix(href, "http://"):
			fmt.Fprintf(l.Debug, "[%s] [%s] not implemented yet\n", time.Now().UTC().Format(time.RFC3339), "ParseHREF")
		case strings.HasPrefix(href, "https://"):
			fmt.Fprintf(l.Debug, "[%s] [%s] not implemented yet\n", time.Now().UTC().Format(time.RFC3339), "ParseHREF")
		}
	}
	return extraURLs
}

func (l *Limiter) ReadResults() {
	for {
		select {
		case s := <-l.Successes:
			l.WaitGroup.Done()
			fmt.Fprintf(l.Stdout, "[%s] [%s] result => URL: %s State: %s Code: %d Error: %v\n", time.Now().UTC().Format(time.RFC3339), "Run", s.URL, s.State, s.ResponseCode, s.Error)
			l.ResultSuccess = append(l.ResultSuccess, s)

		case f := <-l.Fails:
			l.WaitGroup.Done()
			fmt.Fprintf(l.Stdout, "[%s] [%s] result => URL: %s State: %s Code: %d Error: %v\n", time.Now().UTC().Format(time.RFC3339), "Run", f.URL, f.State, f.ResponseCode, f.Error)
			l.ResultFail = append(l.ResultFail, f)
		}
	}
}

func Check(site string, opts ...Option) []*Result {
	l := &Limiter{
		Debug:         io.Discard,
		ResultFail:    []*Result{},
		HTTPClient:    http.Client{},
		Items:         map[string]Rate{},
		Rate:          Rate{Count: 0, MaxRun: 1, Start: time.Time{}, Interval: 1 * time.Second, MaxWait: 3 * time.Second},
		Recursive:     true,
		ResultSuccess: []*Result{},
		Successes:     make(chan *Result),
		Fails:         make(chan *Result),
		Stdout:        os.Stdout,
	}
	for _, o := range opts {
		o(l)
	}

	if l.Quite {
		l.Debug = io.Discard
		l.Stdout = io.Discard
	}
	checked := &Checked{
		Items: map[string]struct{}{},
	}
	go l.ReadResults()

	l.WaitGroup.Add(1)
	checked.ExistOrAdd(site)
	go l.Fetch(site, checked)
	l.WaitGroup.Wait()

	return l.ResultFail
}

func WithHTTPClient(client *http.Client) Option {
	return func(l *Limiter) {
		l.HTTPClient = *client
	}
}

func WithRecursive(b bool) Option {
	return func(l *Limiter) {
		l.Recursive = b
	}
}

func WithStdout(w io.Writer) Option {
	return func(l *Limiter) {
		l.Stdout = w
	}
}

func WithDebug(w io.Writer) Option {
	return func(l *Limiter) {
		l.Debug = w
	}
}

func WithQuite(quite bool) Option {
	return func(l *Limiter) {
		l.Quite = quite
	}
}

func WithRate(intervalSec int, max int, maxWaitSec int) Option {
	return func(l *Limiter) {
		l.Rate = Rate{
			Interval: time.Duration(intervalSec) * time.Second,
			MaxRun:   max,
			MaxWait:  time.Duration(maxWaitSec) * time.Second,
			Start:    time.Time{},
		}
	}
}
