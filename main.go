package main

import (
	"context"
	"log"
	"net/http"
	"reflect"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// The commit hash. Must be populated by `go build -ldflags="-X main.CommitHash=...`
var CommitHash string = "UNKNOWN"

func printRoutes(r chi.Routes) {
	log.Printf("The following routes are recognized:")
	chi.Walk(r, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		var isAuthenticated bool = false
		for _, mw := range middlewares {
			sf1 := reflect.ValueOf(JWTAuthMiddleware)
			sf2 := reflect.ValueOf(mw)
			if sf1 == sf2 {
				isAuthenticated = true
				break
			}
		}

		log.Printf("%-6s => %s (authenticated: %t)", method, route, isAuthenticated)
		return nil
	})
}

func createRouter(repo *MongoRepository) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.AllowContentType("application/json"))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		resp := NewApiErrorResponse(ApiErrorCodeNotFound, "No handler is found for request URI '%s'", r.URL.Path)
		render.Render(w, r, &resp)
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		resp := NewApiErrorResponse(ApiErrorCodeMethodNotAllowed, "The method %s is not allowed on the endpoint '%s'", r.Method, r.URL.Path)
		render.Render(w, r, &resp)
	})

	r.Post("/v1/login", ApiLogin(repo))
	r.Post("/v1/user", ApiCreateUser(repo))
	r.Group(func(r chi.Router) {
		r.Use(JWTAuthMiddleware)
		r.Get("/v1/cv", ApiGetUserCvData(repo))
		r.Post("/v1/cv", ApiPostUserCvData(repo))
		r.Get("/v1/cv/revisions", ApiGetRevisions(repo))
		r.Post("/v1/cv/preview", ApiPostPreview())
		r.Delete("/v1/cv/revisions", ApiDeleteAllRevisions(repo))
		r.Delete("/v1/cv/revisions/{version}", ApiDeleteSingleRevision(repo))
	})

	printRoutes(r)

	return r
}

func main() {
	log.Printf("This is Vita, the CV generator backend (commit %s)", CommitHash)

	uri := "mongodb://localhost:27017"
	opts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(opts)
	if err != nil {
		panic(err)
	}

	if err := client.Ping(context.TODO(), nil); err != nil {
		log.Printf("WARNING: ")
	}
	log.Printf("Connected to '%s'", uri)

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			log.Panicf("Could not disconnect from MongoDB: %s", err)
		}
	}()

	repo := NewMongoRepository(client)
	r := createRouter(&repo)

	log.Printf("Starting webserver on :8080")
	http.ListenAndServe(":8080", r)
}

// https://pkg.go.dev/go.mongodb.org/mongo-driver/v2#section-readme
