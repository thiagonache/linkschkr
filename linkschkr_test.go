package linkschkr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCrawler(t *testing.T) {
	t.Parallel()

	nWorkers := 1000 // just for fun
	work := make(chan *Work)
	results := make(chan *Result)

	for n := 0; n < nWorkers; n++ {
		go Crawler(n, work)
	}

	body := `<html>
	<head>
		<title>For the love of Go!</title>
	</head>
	<body>
		Hi folks,

		We do love <a href="https://golang.org">Go</a>. Go is awesome!
		ajkwerjakwerjkaewrhjakwehrjkawe
		aelwrkhakelrhalkewrhkleawlhrwaklerhalkewrhalewrk

		Visit <a href="https://www.uol.com.br">UOL</a>.
		aewrjaewporawe

		awerjaioewrhaewoi
		a href="testintentional.com">trying to get you in trouble</a>
	</body>
</html>`

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Content-Type", "application/text")
				w.Write([]byte(body))
			}
		}),
	)
	defer ts.Close()

	work <- &Work{
		site:   ts.URL,
		result: results,
	}

	result := <-results
	wantUp := true
	gotUp := result.up
	if wantUp != gotUp {
		t.Errorf("want site up %t, got %t", wantUp, gotUp)
	}

	wantSites := []string{"https://golang.org", "https://www.uol.com.br"}
	gotSites := result.extraURLs

	if !cmp.Equal(wantSites, gotSites) {
		t.Errorf(cmp.Diff(wantSites, gotSites))
	}
}
