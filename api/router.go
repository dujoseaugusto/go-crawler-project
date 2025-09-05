package api

import (
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "go-crawler-project/api/handler"
)

func SetupRouter() *chi.Mux {
    r := chi.NewRouter()
    
    r.Use(middleware.Logger)
    
    r.Get("/properties", handler.GetProperties)
    
    return r
}