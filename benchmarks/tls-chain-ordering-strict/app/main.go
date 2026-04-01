package main

import (
	"fmt"
	"net/http"
	"os"

	"tls-chain-server/tlssetup"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tlsserver <command>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  serve    Start the HTTPS server (blocks)")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		startServer()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func startServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "TLS Chain Server running")
	})

	tlsConfig, err := tlssetup.LoadTLSConfig("/certs/chain.pem", "/certs/leaf.key")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load TLS config: %v\n", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:      ":8443",
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	fmt.Println("Starting HTTPS server on :8443")
	// TLS certs and key are already loaded into TLSConfig, so we pass empty strings.
	if err := server.ListenAndServeTLS("", ""); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
