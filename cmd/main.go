package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/telf01/puncher/pkg/puncher/handlers"
)

var PORT string

func init() {
	PORT = os.Getenv("PUNCHER_PORT")
	if PORT == "" {
		PORT = "9090"
	}
}

func main() {
	l := log.Default()
	master := handlers.NewMaster(l)

	// Create new mux router.
	sm := mux.NewRouter()

	// Handle GET requests.
	getR := sm.Methods(http.MethodGet).Subrouter()
	getR.HandleFunc("/ask", master.HandleAsk)

	s := http.Server{
		Addr:         ":" + PORT, // configure the bind address
		Handler:      sm,                  // set the default handler
		ErrorLog:     master.L,            // set the logger for the server
		ReadTimeout:  5 * time.Second,     // max time to read request from the client
		WriteTimeout: 10 * time.Second,    // max time to write response to the client
		IdleTimeout:  120 * time.Second,   // max time for connections using TCP Keep-Alive
	}

	go func() {
		l.Println("Starting server on", s.Addr)
		err := s.ListenAndServe()
		if err != nil {
			l.Fatal(err)
		}
	}()

	// trap sigterm or interrupt and gracefully shutdown the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Block until a signal is received.
	sig := <-c
	log.Println("Got signal:", sig)

	// gracefully shutdown the server, waiting max 30 seconds for current operations to complete
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	s.Shutdown(ctx)
}
