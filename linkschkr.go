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

type Result struct {
	Error        error
	Refer        string
	ResponseCode int
	State        string
	URL          string
}

type Option func(*Links)

type Links struct {
	Debug         io.Writer
	Fails         chan *Result
	HTTPClient    http.Client
	Interval      time.Duration
	Quite         bool
	Recursive     bool
	ResultFail    []*Result
	ResultSuccess []*Result
	Stdout        io.Writer
	Successes     chan *Result
	WaitGroup     sync.WaitGroup
	Work          Work
}

func Logger(w io.Writer, component string, msg string) {
	fmt.Fprintf(w, "[%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), component, msg)
}

func (l *Links) DoRequest(method, site string, client *http.Client) (*http.Response, error) {
	client.Timeout = l.Interval * 2
	req, err := http.NewRequest(method, site, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "Linkschkr 0.0.1 Beta")
	req.Header.Set("accept", "*/*")
	resp, err := client.Do(req)

	return resp, err
}

func (l *Links) Fetch(work Work, c *Checked, limiter *time.Ticker) {
	Logger(l.Debug, "Fetcher", "started")
	<-limiter.C
	client := &l.HTTPClient
	Logger(l.Debug, "Fetcher", fmt.Sprintf("checking site %s", work.site))
	result := &Result{URL: work.site, Refer: work.refer}
	resp, err := l.DoRequest("HEAD", work.site, client)
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
	Logger(l.Debug, "Fetcher", fmt.Sprintf("Content type %s", ct))
	if !strings.HasPrefix(ct, "text/html") {
		result.State = "up"
		l.Successes <- result
		return
	}
	Logger(l.Debug, "Fetcher", "Run GET method")
	resp, err = l.DoRequest("GET", work.site, client)
	if err != nil {
		result.State = "unkown"
		result.Error = err
		l.Fails <- result
		return
	}
	result.ResponseCode = resp.StatusCode
	Logger(l.Debug, "Fetcher", fmt.Sprintf("response code %d", resp.StatusCode))
	Logger(l.Debug, "Fetcher", "done")
	if resp.StatusCode != http.StatusOK {
		result.State = "down"
		l.Fails <- result
		return
	}
	if l.Recursive {
		extraSites, err := l.ParseBody(resp.Body, work.site)
		if err != nil {
			Logger(l.Stdout, "Fetcher", "error looking for extra sites")
		}
		for _, s := range extraSites {
			exist := c.ExistOrAdd(s)
			if !exist {
				l.WaitGroup.Add(1)
				go l.Fetch(Work{site: s, refer: work.site}, c, limiter)
			}
		}
	}
	result.State = "up"
	l.Successes <- result
}

func (l *Links) ParseBody(r io.Reader, site string) ([]string, error) {
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
			Logger(l.Debug, "ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "/"):
			href = strings.TrimSuffix(href, "/")
			// I'm sure it is a valid URL because it was validated before I just
			// need to parse it again. This is why i'm ignoring error returned
			// from the url.Parse function
			u, _ := url.Parse(site)
			baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			extraURLs = append(extraURLs, fmt.Sprintf("%s%s", baseURL, href))
		case strings.HasPrefix(href, "http://"):
			Logger(l.Debug, "ParseBody", "not implemented yet")
		case strings.HasPrefix(href, "https://"):
			Logger(l.Debug, "ParseBody", "not implemented yet")
		}
	}
	return extraURLs, nil
}

func (l *Links) ReadResults() {
	for {
		select {
		case s := <-l.Successes:
			Logger(l.Debug, "ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", s.URL, s.State, s.ResponseCode, s.Refer, s.Error))
			l.ResultSuccess = append(l.ResultSuccess, s)
			l.WaitGroup.Done()
		case f := <-l.Fails:
			Logger(l.Debug, "ReadResults", fmt.Sprintf("result => URL: %s State: %s Code: %d Refer: %s Error: %v", f.URL, f.State, f.ResponseCode, f.Refer, f.Error))
			l.ResultFail = append(l.ResultFail, f)
			l.WaitGroup.Done()
		}
	}
}

func Check(site string, opts ...Option) []*Result {
	l := &Links{
		Debug:         io.Discard,
		Interval:      1 * time.Second,
		ResultFail:    []*Result{},
		HTTPClient:    http.Client{},
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
	limiter := time.NewTicker(l.Interval)
	l.WaitGroup.Add(1)
	checked.ExistOrAdd(site)
	go l.Fetch(Work{site: site}, checked, limiter)
	l.WaitGroup.Wait()

	return l.ResultFail
}

func WithHTTPClient(client *http.Client) Option {
	return func(l *Links) {
		l.HTTPClient = *client
	}
}

func WithNoRecursion(b bool) Option {
	return func(l *Links) {
		l.Recursive = !b
	}
}

func WithStdout(w io.Writer) Option {
	return func(l *Links) {
		l.Stdout = w
	}
}

func WithDebug(w io.Writer) Option {
	return func(l *Links) {
		l.Debug = w
	}
}

func WithQuite(quite bool) Option {
	return func(l *Links) {
		l.Quite = quite
	}
}
