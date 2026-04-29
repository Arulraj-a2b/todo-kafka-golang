package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// New returns a Gin handler that reverse-proxies to target, stripping stripPrefix
// from the inbound path before forwarding. Example:
//
//	New("http://localhost:8000", "/api/auth")  // /api/auth/login → /login
//	New("http://localhost:8001", "/api")        // /api/todos → /todos
func New(target, stripPrefix string) gin.HandlerFunc {
	u, err := url.Parse(target)
	if err != nil {
		log.Fatalf("invalid proxy target %q: %v", target, err)
	}

	rp := httputil.NewSingleHostReverseProxy(u)

	defaultDirector := rp.Director
	rp.Director = func(req *http.Request) {
		defaultDirector(req)
		if stripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
			if !strings.HasPrefix(req.URL.Path, "/") {
				req.URL.Path = "/" + req.URL.Path
			}
		}
		req.Host = u.Host
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error %s %s -> %s: %v", r.Method, r.URL.Path, target, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream_unavailable"}`))
	}

	return func(c *gin.Context) {
		rp.ServeHTTP(c.Writer, c.Request)
	}
}
