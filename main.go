package main

import (
	"fmt"
	"log"
	"net/http"
)

func HandleAPIRequest(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	default:
		fmt.Fprint(w, "unknown url path")
	case StartsWith(path, "/auth"):
		switch path[len("/auth"):] {
		default:
			fmt.Fprint(w, "unknown url path")
		case "/login":
			UserSigninHandler(w, r)
		}
	case StartsWith(path, "/appeals"):
		switch path[len("/appeals"):] {
		default:
			fmt.Fprint(w, "unknown url path")
		case "/create":
			RequireAuth(AppealsCreateHandler)(w, r)
		case "/all":
			RequireAuth(AppealsGetAllHandler)(w, r)
		}
	}
}

type Router struct{}

func (rt *Router) RouterFunc(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case StartsWith(path, "/api/v1"):
		HandleAPIRequest(w, r, path[len("/api/v1"):])
	}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt.RouterFunc(w, r)
}

func main() {
	if err := OpenDB(); err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer CloseDB()

	router := &Router{}
	log.Println("Server start...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
