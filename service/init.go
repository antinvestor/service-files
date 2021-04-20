package service

import (
	"context"
	"github.com/antinvestor/files/config"
	"github.com/antinvestor/files/service/storage"
	"github.com/gorilla/mux"
	"github.com/pitabwire/frame"
	"log"
	"net/http"
)

func addHandler(service *frame.Service, storageProvider storage.Provider, router *mux.Router,
	f func(w http.ResponseWriter, r *http.Request) error, path string, name string, method string) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		r = r.WithContext(frame.ToContext(r.Context(), service))
		r = r.WithContext(context.WithValue(r.Context(), config.CtxBundleKey, service.Bundle()))
		r = r.WithContext(context.WithValue(r.Context(), config.CtxStorageProviderKey, storageProvider))

		err := f(w, r)
		if err != nil {
			switch e := err.(type) {
			case Error:
				// We can retrieve the status here and write out a specific
				// HTTP status code.
				log.Printf("request failed with  %d - %v", e.Status(), e)
				http.Error(w, e.Error(), e.Status())
			default:

				log.Printf(" request failed with - %v", e)
				// Any error types we don't specifically look out for default
				// to serving a HTTP 500
				http.Error(w, http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
			}
		}

	})

	router.Methods(method).
		Path(path).
		Name(name).
		Handler(handler)

}

// NewRouterV1 -
func NewRouterV1(service *frame.Service, storageProvider storage.Provider) *mux.Router {

	router := mux.NewRouter().StrictSlash(true)

	addHandler(service, storageProvider, router, AddFileV1, "/files", "AddFile", "POST")
	addHandler(service, storageProvider, router, FindFilesV1, "/files", "FindFiles", "GET")
	addHandler(service, storageProvider, router, FindFileByIDV1, "/files/{id}", "FindFileById", "GET")
	addHandler(service, storageProvider, router, DeleteFileV1, "/files/{id}", "DeleteFile", "DELETE")

	return router
}
