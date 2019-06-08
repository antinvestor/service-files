package service

import (
	"time"
	"net/http"
	"os"
	"os/signal"
	"context"
	"github.com/gorilla/handlers"
	"fmt"
)

// Error represents a handler error. It provides methods for a HTTP status
// code and embeds the built-in error interface.
type Error interface {
	error
	Status() int
}

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code int
	Err  error
}

// Allows StatusError to satisfy the error interface.
func (se StatusError) Error() string {
	return se.Err.Error()
}

// Returns our HTTP status code.
func (se StatusError) Status() int {
	return se.Code
}


func RunServer(ctx *ContextV1) {

	waitDuration := time.Second * 15
	router := NewRouterV1(ctx)



	srv := &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%s", ctx.ServerPort ),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      handlers.RecoveryHandler()(router), // Pass our instance of gorilla/mux in.
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			ctx.Logger.Fatalf("Server stopped due to error %v", err)

		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c


	// Create a deadline to wait for.
	ctx2, cancel := context.WithTimeout(context.Background(), waitDuration)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx2)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	ctx.Logger.Infof("Service shutting down at : %v", time.Now())
}