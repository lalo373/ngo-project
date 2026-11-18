package api

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

// staticHandler serves the built SPA from ./static, falling back to
// index.html for client-side routes.
func (a *API) staticHandler() http.Handler {
	fileServer := http.FileServer(http.Dir("static"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		if p == "/" || p == "." {
			p = "/index.html"
		}
		// known SPA routes always serve the shell
		spaRoute := p == "/dashboard" || strings.HasPrefix(p, "/login/") ||
			(strings.HasPrefix(p, "/admin") && !spaAsset(p))
		if spaRoute {
			serveIndex(w, r, fileServer)
			return
		}
		// otherwise serve real files if they exist, else the SPA shell
		if p != "/index.html" {
			if _, err := os.Stat(path.Join("static", p)); err != nil {
				serveIndex(w, r, fileServer)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

// spaAsset returns true for paths under /admin that look like real
// assets (e.g. /admin/assets/app.js) rather than client routes.
func spaAsset(p string) bool {
	if strings.HasPrefix(p, "/admin/assets/") || strings.HasPrefix(p, "/admin/favicon") {
		return true
	}
	return strings.Contains(path.Ext(p), ".") && p != "/admin"
}

func serveIndex(w http.ResponseWriter, r *http.Request, fileServer http.Handler) {
	r2 := r.Clone(r.Context())
	r2.URL = new(url.URL)
	*r2.URL = *r.URL
	r2.URL.Path = "/" // FileServer serves index.html for the root
	r2.RequestURI = ""
	fileServer.ServeHTTP(w, r2)
}
