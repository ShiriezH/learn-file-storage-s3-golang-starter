package main

import "net/http"

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers to prevent caching
		w.Header().Set("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}
