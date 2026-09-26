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

	// Ensure Vercel query string and original path are preserved for Fiber adaptor
	if fwd := r.Header.Get("X-Forwarded-Uri"); fwd != "" {
		if u, err := url.Parse(fwd); err == nil {
			r.URL = u
			r.RequestURI = fwd
		}
	} else if orig := r.Header.Get("X-Matched-Path"); orig != "" && r.URL.RawQuery != "" {
		r.RequestURI = orig + "?" + r.URL.RawQuery
	}

	httpHandler(w, r)
}
