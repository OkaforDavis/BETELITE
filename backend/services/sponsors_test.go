package services

import "testing"

func TestCleanSponsorLink(t *testing.T) {
	ok := map[string]string{
		"":                      "",
		"instagram.com/brand":   "https://instagram.com/brand",
		"https://brand.ng/shop": "https://brand.ng/shop",
		" http://brand.com.gh ": "http://brand.com.gh",
	}
	for in, want := range ok {
		if got, err := cleanSponsorLink(in); err != nil || got != want {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "ftp://x.com", "notalink", "https://localhost/x"} {
		if _, err := cleanSponsorLink(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestVideoType(t *testing.T) {
	mp4 := append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42xxxxxxxx")...)
	webm := []byte{0x1A, 0x45, 0xDF, 0xA3, 1, 2, 3}
	if videoType(mp4) != "video/mp4" || videoType(webm) != "video/webm" || videoType([]byte("<html>hello</html>")) != "" {
		t.Fatal("video detection wrong")
	}
}
