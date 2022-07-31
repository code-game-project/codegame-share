package main

import (
	"fmt"
	"net/http"

	"github.com/Bananenpro/log"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/pflag"
)

type Server struct {
	Router chi.Router
	DB     *sqlx.DB
}

func (s *Server) run(port int) error {
	address := fmt.Sprintf(":%d", port)
	log.Infof("Listening on %s...", address)
	return http.ListenAndServe(address, s.Router)
}

func main() {
	log.SetSeverity(log.INFO)
	var port int
	pflag.IntVarP(&port, "port", "p", 8080, "The port of the server.")
	var database string
	pflag.StringVarP(&database, "database", "d", "database.sqlite", "The path to the sqlite database.")
	pflag.Parse()

	db, err := connectDB(database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %s", err)
	}
	s := &Server{
		Router: chi.NewRouter(),
		DB:     db,
	}
	s.registerRoutes()
	log.Fatal(s.run(port))
}
