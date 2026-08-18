// Package main is the entry point for the gcs backend server.
package main

import (
	"log"
	"net/http"
	"os"

	"yalb.gcs/internal/codec"
)

func main() {
	// The transport trust posture is printed before anything binds. Tier 5
	// opens the socket; the warning belongs at startup either way, because a
	// default nobody is told about is a default nobody revisits.
	bind := codec.ResolveBind(os.LookupEnv(codec.EnvUDPBind))
	if warning := codec.PostureWarning(bind, os.Getenv(codec.EnvSigningKey)); warning != "" {
		log.Println(warning)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Println("gcs backend listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
