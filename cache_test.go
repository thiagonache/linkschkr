package links_test

import (
	"links"
	"testing"
)

func TestGetCache(t *testing.T) {
	t.Parallel()
	url := "https://bitfieldconsulting.com"
	cache := links.NewCache()
	value, ok := cache.Get(url)
	if ok {
		t.Fatalf("cache should empty but %q was found", value)
	}
	want := "johniscool"
	cache.Store(url, want)
	got, ok := cache.Get(url)
	if !ok {
		t.Fatal("fail to retrieve stored value")
	}
	if want != got {
		t.Errorf("want %q but got %q", want, got)
	}
}
