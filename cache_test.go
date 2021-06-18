package links_test

import (
	"links"
	"testing"
)

type StubCacheStore struct {
	Data map[string]string
}

func (s *StubCacheStore) GetData(key string) string {
	return s.Data[key]
}

func TestGetCache(t *testing.T) {
	testCases := []struct {
		desc, url, want string
	}{
		{
			desc: "returns bitfield's website cache data",
			url:  "https://bitfieldconsulting.com",
			want: "up",
		},
		{
			desc: "returns java's website cache data",
			url:  "https://java.com",
			want: "down",
		},
		{
			desc: "returns empty for unknown website cache data",
			url:  "https://thiagonbcarvalho.com",
			want: "",
		},
	}
	data := StubCacheStore{
		Data: map[string]string{
			"https://bitfieldconsulting.com": "up",
			"https://java.com":               "down",
			"https://thiagonbcarvalho.com":   "",
		},
	}
	cache := &links.CacheServer{&data}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got := cache.GetCache(tC.url)
			if tC.want != got {
				t.Errorf("want %q, got %q", tC.want, got)
			}
		})
	}
}
