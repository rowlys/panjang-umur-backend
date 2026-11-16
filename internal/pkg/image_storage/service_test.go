package image_storage

import "testing"

func TestKeyFromURL(t *testing.T) {
	service := &service{bucketURL: "https://cdn.example.com"}

	cases := map[string]string{
		"https://cdn.example.com/avatars/user/photo.jpg": "avatars/user/photo.jpg",
		"https://cdn.example.com.evil/avatars/x.jpg":     "",
		"https://other.example.com/avatars/x.jpg":        "",
		"":                                                "",
	}

	for url, want := range cases {
		if got := service.KeyFromURL(url); got != want {
			t.Errorf("KeyFromURL(%q) = %q, want %q", url, got, want)
		}
	}
}
