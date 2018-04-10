package service

import (
	"context"
	"net/http"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/unrolled/secure"
	"github.com/urfave/negroni"
)

func (svc *Service) MakeHTTPHandler(ctx context.Context, logger log.Logger) http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/health", svc.HealthCheckHandler).Methods("GET")

	isDev := os.Getenv("ENV") == "development"

	n := negroni.New()

	recovery := negroni.NewRecovery()
	recovery.PrintStack = false
	n.Use(recovery)

	nLogger := negroni.NewLogger()
	n.Use(nLogger)

	secureMiddleware := secure.New(secure.Options{
		IsDevelopment: isDev,
	})
	n.Use(negroni.HandlerFunc(secureMiddleware.HandlerFuncWithNext))

	corsMiddleware := cors.New(cors.Options{
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"Accept", "content-type", "Content-Length", "Accept-Encoding"},
	})
	n.Use(corsMiddleware)

	n.UseHandler(r)

	return n
}
