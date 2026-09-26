package handler

import (
	"net/http"
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
	httpHandler(w, r)
}
