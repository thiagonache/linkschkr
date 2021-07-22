package links_test

import (
	"links"
	"testing"
	"time"
)

func TestGetCache(t *testing.T) {
	t.Parallel()
	url := "https://bitfieldconsulting.com"
	cache := links.NewCache(24 * time.Hour)
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

func TestGetCacheExpiration(t *testing.T) {
	t.Parallel()
	url := "https://golang.org"
	cache := links.NewCache(100 * time.Microsecond)
	got, ok := cache.Get(url)
	if ok {
		t.Fatalf("cache should empty but %q was found", got)
	}
	want := "johniscool"
	cache.Store(url, want)
	got, ok = cache.Get(url)
	if !ok {
		t.Fatal("fail to retrieve stored value")
	}
	if want != got {
		t.Errorf("want %q but got %q", want, got)
	}
	time.Sleep(100 * time.Microsecond)
	got, ok = cache.Get(url)
	if ok {
		t.Fatalf("cache should empty but %q was found", got)
	}
}
