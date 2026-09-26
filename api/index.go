package handler

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/yourusername/ecom-backend/config"
	"github.com/yourusername/ecom-backend/pkg/serverapp"
)

var (
	httpHandler http.HandlerFunc
	once        sync.Once
)

func initApp() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		cfg, _ = config.Load()
	}
	app, err := serverapp.BuildApp(cfg)
	if err != nil {
		panic(err)
	}
	httpHandler = adaptor.FiberApp(app)
}

// Handler is the Vercel Serverless Function entry point.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)

	// Ensure Vercel query string is preserved for Fiber adaptor
	if r.URL.RawQuery == "" {
		if reqURI := r.RequestURI; reqURI != "" {
			if u, err := url.ParseRequestURI(reqURI); err == nil && u.RawQuery != "" {
				r.URL.RawQuery = u.RawQuery
			}
		}
		if fwd := r.Header.Get("X-Forwarded-Uri"); fwd != "" {
			if u, err := url.Parse(fwd); err == nil && u.RawQuery != "" {
				r.URL.RawQuery = u.RawQuery
			}
		}
	} else if fwd := r.Header.Get("X-Forwarded-Uri"); fwd != "" {
		if u, err := url.Parse(fwd); err == nil {
			r.URL.Path = u.Path
			if u.RawQuery != "" && r.URL.RawQuery == "" {
				r.URL.RawQuery = u.RawQuery
			}
		}
	}

	httpHandler(w, r)
}
