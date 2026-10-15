package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
)

func TestAmazonLoginBoundary(t *testing.T) {
	for _, host := range []string{"amazon.com", ".amazon.ca", "www.amazon.co.uk", "www.amazon.co.jp", "photos.amazon.de"} {
		if !amazonCookieDomain(host) {
			t.Errorf("rejected Amazon host %s", host)
		}
	}
	for _, host := range []string{"amazon.com.evil.test", "fakeamazon.com", "localhost", "127.0.0.1", "amazon.invalid"} {
		if amazonCookieDomain(host) {
			t.Errorf("trusted other host %s", host)
		}
	}
	rc, _ := aphotos.ResolveRegion(aphotos.RegionCA)
	cookies := make([]mygo.Cookie, 0, 5)
	cookies = append(cookies, []mygo.Cookie{{Name: "session-id", Value: "sid", HTTPOnly: true}, {Name: "ubid-acbca", Value: "ubid"}, {Name: "at-acbca", Value: "secret", HTTPOnly: true}, {Name: "unneeded", Value: "omit"}}...)
	got, err := loginCookies(rc, cookies)
	if err != nil || len(got) != 3 || got["at-acbca"] != "secret" {
		t.Fatalf("capture: %d %v", len(got), err)
	}
	got, err = loginCookies(rc, cookies[:2])
	if err != nil || got != nil {
		t.Fatal("incomplete session accepted")
	}
	cookies = append(cookies, mygo.Cookie{Name: "at-acbca", Value: "conflicting"})
	if _, err := loginCookies(rc, cookies); err == nil {
		t.Fatal("ambiguous session accepted")
	}
	cookies[2].Expires = time.Now().Add(-time.Hour)
	got, err = loginCookies(rc, cookies[:4])
	if err != nil || got != nil {
		t.Fatal("expired session accepted")
	}
}
