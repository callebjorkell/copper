package main

import (
	"fmt"
	"net/http"
	"os"
)

func handlerPing(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(`{"message": "pong"}`))
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func main() {
	err := StartServer()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func StartServer() error {
	http.HandleFunc("/ping", handlerPing)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return fmt.Errorf("could not start server: %w", err)
	}
	return nil
}
