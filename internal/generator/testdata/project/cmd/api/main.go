// Command api is the HTTP entry point for testapp.
package main

import (
	"errors"
	"net/http"
	"os"
	"time"

	"example.com/testapp/internal/platform/container"
	"example.com/testapp/internal/platform/router"
)

// readHeaderTimeout bounds how long a client may take to send its headers.
const readHeaderTimeout = 5 * time.Second

func main() {
	c := container.New()

	srv := &http.Server{
		Addr:              ":" + envOr("PORT", "8080"),
		Handler:           router.New(c),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	c.Logger.Info("testapp listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		c.Logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// envOr returns the value of key, or fallback when the variable is unset or
// empty.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
