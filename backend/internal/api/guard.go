package api

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// localOnly protects the login-less API (spec §17) from other sites open in the browser:
//   - the Host must be localhost, 127.0.0.1 or [::1] (any port), which defeats DNS rebinding;
//   - requests that change something need a local Origin when one is sent and, when they carry
//     a body, a JSON or multipart body type, which a cross-site form cannot send without a
//     CORS preflight.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			writeError(w, http.StatusForbidden, "acesso permitido só por localhost")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" && !isLocalOrigin(origin) {
				writeError(w, http.StatusForbidden, "origem não permitida")
				return
			}
			// A bodyless write (skip, suggest, delete) carries nothing to forge; cross-site browser
			// requests are already stopped by the Origin check above.
			bodyless := r.ContentLength == 0 && len(r.TransferEncoding) == 0
			mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if !bodyless && mt != "application/json" && mt != "multipart/form-data" {
				writeError(w, http.StatusUnsupportedMediaType, "envie Content-Type application/json ou multipart/form-data")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isLocalHost reports whether a Host header ("host" or "host:port") names this machine.
func isLocalHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func isLocalOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return isLocalHost(u.Host)
}
