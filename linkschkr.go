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
	Result     chan int
	Stdout     io.Writer
}

type Rate struct {
	Count    int
	Interval time.Duration
	Max      int
	MaxWait  time.Duration
	Start    time.Time
}

type Counter struct {
	mu    sync.Mutex
	count int
}

func (r *Counter) Inc() {
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
}

func (r *Counter) Dec() {
	r.mu.Lock()
	r.count--
	r.mu.Unlock()
}

func (r *Counter) Get() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (l *Limiter) Start(checked *Checked, counter *Counter) {
	fmt.Fprintf(l.Debug, "[%s] [%s] starting\n", time.Now().Format(time.RFC3339), "Limiter")
	ticker := time.NewTicker(l.Rate.Interval)
	for {
		<-ticker.C
		fmt.Fprintf(l.Debug, "[%s] [%s] waiting on input channel\n", time.Now().Format(time.RFC3339), "Limiter")
		site := <-l.Input
		fmt.Fprintf(l.Debug, "[%s] [%s] got %s\n", time.Now().Format(time.RFC3339), "Limiter", site)
		fmt.Fprintf(l.Debug, "[%s] [%s] checking if site was already checked\n", time.Now().Format(time.RFC3339), "Limiter")
		exist := checked.Get(site)
		for exist {
			fmt.Fprintf(l.Debug, "[%s] [%s] already checked\n", time.Now().Format(time.RFC3339), "Limiter")
			fmt.Fprintf(l.Debug, "[%s] [%s] waiting on input channel\n", time.Now().Format(time.RFC3339), "Limiter")
			site = <-l.Input
			fmt.Fprintf(l.Debug, "[%s] [%s] got %s\n", time.Now().Format(time.RFC3339), "Limiter", site)
			fmt.Fprintf(l.Debug, "[%s] [%s] checking if site was already checked\n", time.Now().Format(time.RFC3339), "Limiter")
			exist = checked.Get(site)
		}
		fmt.Fprintf(l.Debug, "[%s] [%s] adding %s to sites already checked\n", time.Now().Format(time.RFC3339), "Limiter", site)
		checked.Add(site)
		fmt.Fprintf(l.Debug, "[%s] [%s] incrementing running fetchers\n", time.Now().Format(time.RFC3339), "Limiter")
		counter.Inc()
		fmt.Fprintf(l.Debug, "[%s] [%s] start fetcher goroutine\n", time.Now().Format(time.RFC3339), "Limiter")
		go l.Fetcher(site, counter, checked)
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

func (l *Limiter) Fetcher(site string, w *Counter, c *Checked) {
	defer w.Dec()
	fmt.Fprintf(l.Debug, "[%s] [%s] started\n", time.Now().Format(time.RFC3339), "Fetcher")
	// it costs a lock for no reason when debug is disable. To be reconsidered.
	//fmt.Fprintf(l.Debug, "[%s] [%s] running %d fetchers\n", time.Now().Format(time.RFC3339), "Fetcher", w.Get())
	client := &l.HTTPClient
	fmt.Fprintf(l.Stdout, "[%s] [%s] checking site %s\n", time.Now().Format(time.RFC3339), "Fetcher", site)
	resp, err := l.DoRequest("HEAD", site, client)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Fprintf(c.Debug, "[%v] [%s] Response code %d\n", time.Now().Format(time.RFC3339), site, resp.StatusCode)
	//result.StatusCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		if !l.Recursive {
			l.Quit <- struct{}{}
			return
		}
		return
	}
	ct := resp.Header.Get("Content-Type")
	//fmt.Fprintf(c.Debug, "[%v] [%s] Content type %s\n", time.Now().Format(time.RFC3339), site, ct)
	if !strings.HasPrefix(ct, "text/html") {
		if !l.Recursive {
			l.Quit <- struct{}{}
			return
		}
		return
	}
	//fmt.Fprintf(c.Debug, "[%v] [%s] Run GET method\n", time.Now().Format(time.RFC3339), site)
	resp, err = l.DoRequest("GET", site, client)
	if err != nil {
		if !l.Recursive {
			l.Quit <- struct{}{}
			return
		}
		return
	}
	//fmt.Fprintf(c.Debug, "[%v] [%s] Response code %d\n", time.Now().Format(time.RFC3339), site, resp.StatusCode)
	//result.StatusCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		if !l.Recursive {
			l.Quit <- struct{}{}
			return
		}
		return
	}
	fmt.Fprintf(l.Stdout, "[%s] [%s] Done\n", time.Now().Format(time.RFC3339), "Fetcher")

	if !l.Recursive {
		l.Quit <- struct{}{}
		return
	}
	extraSites := l.ParseHREF(resp.Body, site)
	for _, s := range extraSites {
		go l.SendWork(s, c)
	}
	//fmt.Fprintf(c.Debug, "[%v] [%s] Extra sites %q\n", time.Now().Format(time.RFC3339), site, extraSites)
	//result.ExtraSites = append(result.ExtraSites, extraSites...)

	l.Result <- resp.StatusCode

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
			// case strings.HasPrefix(href, "http://"):
			// 	extraURLs = append(extraURLs, href)
			// case strings.HasPrefix(href, "https://"):
			// 	extraURLs = append(extraURLs, href)
		}
	}
	return extraURLs
}

func Run(site string, opts ...Option) {
	l := &Limiter{
		Debug: io.Discard,
		Input: make(chan string),
		Items: map[string]Rate{},
		Quit:  make(chan struct{}),
		Quite: false,
		Rate: Rate{
			Count:    0,
			Max:      20,
			Start:    time.Time{},
			Interval: 5 * time.Second,
			MaxWait:  10 * time.Second,
		},
		Recursive: true,
		Result:    make(chan int),
		Stdout:    os.Stdout,
	}
	for _, o := range opts {
		o(l)
	}
	if l.Quite {
		l.Debug = io.Discard
		l.Stdout = io.Discard
	}
	c := &Checked{
		Items: map[string]struct{}{},
	}
	w := &Counter{}
	for x := 0; x < l.Rate.Max; x++ {
		go l.Start(c, w)
	}
	go l.SendWork(site, c)
	for {
		select {
		case r := <-l.Result:
			fmt.Println(r)
		case <-l.Quit:
			return
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(l *Limiter) {
		l.HTTPClient = *client
	}
}

func WithRunRecursively(b bool) Option {
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
