package router

import (
	handlers "dataverification/handlers"

	"github.com/gorilla/mux"
)

func NewRouter(handlers *handlers.Handler) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/verify_data", handlers.VerifyConnectedSellers).Methods("GET")

	return r
}
