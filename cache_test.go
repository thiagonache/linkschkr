package links_test

import (
	"links"
	"testing"
)

// func TestGetCache(t *testing.T) {
// 	testCases := []struct {
// 		desc, url, want string
// 	}{
// 		{
// 			desc: "returns bitfield's website cache data",
// 			url:  "https://bitfieldconsulting.com",
// 			want: "up",
// 		},
// 		{
// 			desc: "returns java's website cache data",
// 			url:  "https://java.com",
// 			want: "down",
// 		},
// 		{
// 			desc: "returns empty for unknown website cache data",
// 			url:  "https://thiagonbcarvalho.com",
// 			want: "",
// 		},
// 	}
// 	data := links.CacheServer{
// 		Data: map[string]string{
// 			"https://bitfieldconsulting.com": "up",
// 			"https://java.com":               "down",
// 			"https://thiagonbcarvalho.com":   "",
// 		},
// 	}
// 	cache := &links.CacheServer{&data}
// 	for _, tC := range testCases {
// 		t.Run(tC.desc, func(t *testing.T) {
// 			got := cache.GetCache(tC.url)
// 			if tC.want != got {
// 				t.Errorf("want %q, got %q", tC.want, got)
// 			}
// 		})
// 	}
// }

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
