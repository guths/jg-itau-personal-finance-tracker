package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalGuard(t *testing.T) {
	a := newAPI(t)
	cases := []struct {
		name, method, path, host, ctype, origin string
		want                                    int // 0 = anything but 403/415
	}{
		{"host externo", "GET", "/api/months", "example.com", "", "", 403},
		{"rebinding", "GET", "/api/months", "evil.localhost.com:8080", "", "", 403},
		{"outro loopback", "GET", "/api/months", "127.0.0.2:8080", "", "", 403},
		{"localhost", "GET", "/api/months", "localhost:5173", "", "", 200},
		{"localhost sem porta", "GET", "/api/months", "localhost", "", "", 200},
		{"127.0.0.1", "GET", "/api/months", "127.0.0.1:8080", "", "", 200},
		{"ipv6", "GET", "/api/months", "[::1]:8080", "", "", 200},
		{"sem content-type", "POST", "/api/questions/suggest", "localhost:8080", "", "", 415},
		{"text/plain", "POST", "/api/questions/suggest", "localhost:8080", "text/plain", "", 415},
		{"form urlencoded", "DELETE", "/api/rules/1", "localhost:8080", "application/x-www-form-urlencoded", "", 415},
		{"origem externa", "POST", "/api/questions/suggest", "localhost:8080", "application/json", "http://evil.com", 403},
		{"origem null", "POST", "/api/questions/suggest", "localhost:8080", "application/json", "null", 403},
		{"origem local", "POST", "/api/questions/suggest", "localhost:8080", "application/json; charset=utf-8", "http://localhost:5173", 0},
		{"multipart", "POST", "/api/imports", "127.0.0.1:8080", "multipart/form-data; boundary=x", "http://127.0.0.1:5173", 0},
		{"delete json", "DELETE", "/api/rules/999", "localhost:8080", "application/json", "", 0},
		{"get com origem externa", "GET", "/api/months", "localhost:8080", "", "http://evil.com", 200},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(""))
		req.Host = c.host
		if c.ctype != "" {
			req.Header.Set("Content-Type", c.ctype)
		}
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		switch {
		case c.want == 0 && (rec.Code == http.StatusForbidden || rec.Code == http.StatusUnsupportedMediaType):
			t.Errorf("%s: %d %s", c.name, rec.Code, rec.Body.String())
		case c.want != 0 && rec.Code != c.want:
			t.Errorf("%s: %d, want %d (%s)", c.name, rec.Code, c.want, rec.Body.String())
		}
		if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnsupportedMediaType {
			if !strings.Contains(rec.Body.String(), `"error"`) {
				t.Errorf("%s: body %s", c.name, rec.Body.String())
			}
		}
	}
}
