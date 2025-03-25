package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os/exec"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/spf13/viper"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// The commit hash. Must be populated by `go build -ldflags="-X main.CommitHash=...`
var CommitHash string = "UNKNOWN"

type VitaConfig struct {
	TemplateDirectory string `mapstructure:"template_directory"`
	MongoUri          string `mapstructure:"mongo_uri"`
}

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

func createRouter(cfg VitaConfig, repo *MongoRepository) chi.Router {
	r := chi.NewRouter()

	r.Use(cors.AllowAll().Handler)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.AllowContentType("application/json"))

	apiResource := NewVitaJsonAPIResource(cfg, repo)

	r.Mount("/api/v1", apiResource.Routes())

	printRoutes(r)

	return r
}

func startupCheck() {
	cmd := exec.Command("typst", "--version")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Could not connect stdout pipe for 'typst': %s", err)
	}

	if err = cmd.Start(); err != nil {
		errstr := "Could not start the command 'typst'. It may not be available on your $PATH. " +
			"Typst can be downloaded via https://github.com/typst/typst/releases."
		log.Fatal(strings.TrimSpace(errstr))
	}

	b, err := io.ReadAll(stdout)
	if err != nil {
		log.Fatalf("Could not read from stdout for the 'typst' binary")
	}

	what := strings.Split(string(b), " ")
	if len(what) >= 2 {
		log.Printf("Found 'typst' version %s", what[1])
	}

}

func loadConfig() VitaConfig {
	viper.SetConfigName("vita")
	viper.SetConfigType("toml")
	viper.AddConfigPath("~/.config/vita")
	viper.AddConfigPath("./")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Could not vita config: %s", err)
	}

	m := viper.AllKeys()
	for _, v := range m {
		log.Printf("Found configuration item '%s' = '%s'", v, viper.GetString(v))
	}

	cfg := VitaConfig{}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Fatalf("Could not read vita config: %s", err)
	}

	return cfg
}

func main() {
	log.Printf("This is Vita, the CV generator backend (commit %s)", CommitHash)

	config := loadConfig()
	startupCheck()

	uri := "mongodb://localhost:27017"
	opts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(opts)
	if err != nil {
		panic(err)
	}

	if err := client.Ping(context.TODO(), nil); err != nil {
		log.Printf("WARNING: initial Mongo ping check failed")
	}
	log.Printf("Connected to '%s'", uri)

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			log.Panicf("Could not disconnect from MongoDB: %s", err)
		}
	}()

	repo := NewMongoRepository(client)
	r := createRouter(config, &repo)

	log.Printf("Starting webserver on :8080")
	http.ListenAndServe(":8080", r)
}

// https://pkg.go.dev/go.mongodb.org/mongo-driver/v2#section-readme
