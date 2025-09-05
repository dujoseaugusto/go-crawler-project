package api

import (
	"github.com/dujoseaugusto/go-crawler-project/api/handler"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func SetupRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/properties", handler.GetProperties)

	return r
}
