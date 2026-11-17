package mobile

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// constEq compares two strings in constant time so token checks do not
// leak content through timing side channels.
func constEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// bearerToken extracts the Bearer credential from the Authorization
// header, if present.
func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// isReadMethod reports whether the request only reads server state.
func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// authGuard enforces pairing-token authentication. Writes always require
// the token; reads require it only when ProtectReads is set. An empty
// configured token fails closed with 503 instead of leaving the drive
// writable.
func (s *Server) authGuard() func(http.HandlerFunc) http.Handler {
	return func(next http.HandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			needToken := !isReadMethod(r.Method) || s.opts.ProtectReads
			if !needToken {
				next.ServeHTTP(w, r)
				return
			}
			if s.opts.Token == "" {
				writeErr(w, http.StatusServiceUnavailable, "pairing token not configured")
				return
			}
			if !constEq(bearerToken(r), s.opts.Token) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="painless"`)
				writeErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
