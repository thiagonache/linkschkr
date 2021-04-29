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

func (c *Checked) Add(key string) {
	c.mu.Lock()
	c.Items[key] = struct{}{}
	c.mu.Unlock()
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
	Debug      io.Writer
	HTTPClient http.Client
	Input      chan string
	Items      map[string]Rate
	Quit       chan struct{}
	Quite      bool
	Rate       Rate
	Recursive  bool
	Successes  chan *Result
	Fails      chan *Result
	Stdout     io.Writer
}

type Rate struct {
	Count    int
	Interval time.Duration
	MaxRun   int
	MaxWait  time.Duration
	Start    time.Time
}

func (l *Limiter) Start(checked *Checked) {
	fmt.Fprintf(l.Debug, "[%s] [%s] starting\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
	ticker := time.NewTicker(l.Rate.Interval)
	for {
		fmt.Fprintf(l.Debug, "[%s] [%s] waiting on ticker channel\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
		<-ticker.C
		receiveOrDie := time.NewTicker(l.Rate.Interval + l.Rate.MaxWait)
		fmt.Fprintf(l.Debug, "[%s] [%s] waiting on input channel\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
		select {
		case site := <-l.Input:
			fmt.Fprintf(l.Debug, "[%s] [%s] got %s\n", time.Now().UTC().Format(time.RFC3339), "Limiter", site)
			fmt.Fprintf(l.Debug, "[%s] [%s] checking if site was already checked\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
			exist := checked.Get(site)
			for exist {
				fmt.Fprintf(l.Debug, "[%s] [%s] already checked\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
				fmt.Fprintf(l.Debug, "[%s] [%s] waiting on input channel\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
				site = <-l.Input
				fmt.Fprintf(l.Debug, "[%s] [%s] got %s\n", time.Now().UTC().Format(time.RFC3339), "Limiter", site)
				fmt.Fprintf(l.Debug, "[%s] [%s] checking if site was already checked\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
				exist = checked.Get(site)
			}
			fmt.Fprintf(l.Debug, "[%s] [%s] adding %s to sites already checked\n", time.Now().UTC().Format(time.RFC3339), "Limiter", site)
			checked.Add(site)
			fmt.Fprintf(l.Debug, "[%s] [%s] incrementing running fetchers\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
			fmt.Fprintf(l.Debug, "[%s] [%s] start fetcher goroutine\n", time.Now().UTC().Format(time.RFC3339), "Limiter")
			go l.Fetcher(site, checked)
		case <-receiveOrDie.C:
			l.Quit <- struct{}{}
			return
		}
	}
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

func (l *Limiter) SendWork(site string, c *Checked) {
	exist := c.Get(site)
	if !exist {
		l.Input <- site
	}
}

func (l *Limiter) Fetcher(site string, c *Checked) {
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
		if !l.Recursive {
			l.Quit <- struct{}{}
		}
		return
	}
	ct := resp.Header.Get("Content-Type")
	fmt.Fprintf(l.Debug, "[%v] [%s] Content type %s\n", time.Now().UTC().Format(time.RFC3339), "Fetcher", ct)
	if !strings.HasPrefix(ct, "text/html") {
		if !l.Recursive {
			result.State = "up"
			l.Successes <- result
			l.Quit <- struct{}{}
		}
		return
	}

	fmt.Fprintf(l.Debug, "[%v] [%s] Run GET method\n", time.Now().UTC().Format(time.RFC3339), "Fetcher")
	resp, err = l.DoRequest("GET", site, client)
	if err != nil {
		// should I put it between the request and the error handling?! to be discussed
		result.ResponseCode = resp.StatusCode
		result.State = "unkown"
		result.Error = err
		l.Fails <- result
		if !l.Recursive {
			l.Quit <- struct{}{}
		}
		return
	}
	result.ResponseCode = resp.StatusCode
	fmt.Fprintf(l.Debug, "[%s] [%s] response code %d\n", time.Now().UTC().Format(time.RFC3339), "Fetcher", resp.StatusCode)
	fmt.Fprintf(l.Debug, "[%s] [%s] done\n", time.Now().UTC().Format(time.RFC3339), "Fetcher")
	if resp.StatusCode != http.StatusOK {
		result.State = "down"
		l.Fails <- result
		if !l.Recursive {
			l.Quit <- struct{}{}
		}
		return
	}

	result.State = "up"
	l.Successes <- result
	if !l.Recursive {
		l.Quit <- struct{}{}
		return
	}
	extraSites := l.ParseHREF(resp.Body, site)
	for _, s := range extraSites {
		go l.SendWork(s, c)
	}
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

func Run(site string, opts ...Option) ([]*Result, []*Result) {
	l := &Limiter{
		Debug: io.Discard,
		Fails: make(chan *Result),
		Input: make(chan string),
		Items: map[string]Rate{},
		Quite: false,
		Rate: Rate{
			Count:    0,
			MaxRun:   1,
			Start:    time.Time{},
			Interval: 1 * time.Second,
			MaxWait:  3 * time.Second,
		},
		Recursive: true,
		Stdout:    os.Stdout,
		Successes: make(chan *Result),
	}
	for _, o := range opts {
		o(l)
	}
	l.Quit = make(chan struct{}, l.Rate.MaxRun)
	if l.Quite {
		l.Debug = io.Discard
		l.Stdout = io.Discard
	}
	checked := &Checked{
		Items: map[string]struct{}{},
	}
	for x := 0; x < l.Rate.MaxRun; x++ {
		go l.Start(checked)
	}
	go l.SendWork(site, checked)
	responseSuccess, responseFail := []*Result{}, []*Result{}
	for {
		select {
		case s := <-l.Successes:
			fmt.Fprintf(l.Debug, "[%s] [%s] result => URL: %s State: %s Error: %v\n", time.Now().UTC().Format(time.RFC3339), "Run", s.URL, s.State, s.Error)
			responseSuccess = append(responseSuccess, s)
		case f := <-l.Fails:
			fmt.Fprintf(l.Debug, "[%s] [%s] result => URL: %s State: %s Error: %v\n", time.Now().UTC().Format(time.RFC3339), "Run", f.URL, f.State, f.Error)
			responseFail = append(responseFail, f)
		case <-l.Quit:
			return responseSuccess, responseFail
		}
	}
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
