package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	testMongoClient *mongo.Client
	testRepo        MongoRepository
	testRouter      chi.Router
)

func mustUnmarshal(t *testing.T, rc io.ReadCloser, v any) {
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Errorf("unable to read body: %s", err)
	}
	err = json.Unmarshal(b, v)
	if err != nil {
		t.Errorf("unable to unmarshal: %s", err)
	}
}

func mustRead(rc io.ReadCloser) string {
	bytes, err := io.ReadAll(rc)
	if err != nil {
		panic("could not read body")
	}

	return string(bytes)
}

func TestMain(t *testing.M) {
	testMongoClient = createTestingMongoClient()
	defer testMongoClient.Disconnect(context.TODO())
	testRepo = NewMongoRepository(testMongoClient)
	testRouter = createRouter(&testRepo)
	code := t.Run()
	os.Exit(code)
}

func createTestingMongoClient() *mongo.Client {
	uri := "mongodb://localhost:27017"
	opts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(opts)
	if err != nil {
		panic("Unable to create a connection using the supplied string")
	}

	if err := client.Ping(context.TODO(), nil); err != nil {
		panic("Could not create a connection to the local Mongo database")
	}

	return client
}

func TestMethodNotAllowed(t *testing.T) {
	// GET is not allowed on /v1/user
	req := httptest.NewRequest(http.MethodGet, "/v1/user", nil)
	w := httptest.NewRecorder()

	testRouter.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()

	if w.Result().StatusCode != 405 {
		t.Errorf("expected status code 405")
	}

	var errResponse ApiErrorResponse
	mustUnmarshal(t, w.Result().Body, &errResponse)

	if errResponse.Error.Code != ApiErrorCodeMethodNotAllowed {
		t.Errorf("expected 405 here")
	}
}

func TestCreateUser(t *testing.T) {
	userRequest := CreateUserRequest{
		Username: "testuser",
		Password: "testuser",
	}
	b, _ := json.Marshal(userRequest)
	reader := bytes.NewReader(b)

	req := httptest.NewRequest(http.MethodPost, "/v1/user", reader)

	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	testRouter.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()

	if w.Result().StatusCode != 200 {
		body := mustRead(w.Result().Body)
		t.Errorf("statuscode is not 200 but %d. Response is '%s'", w.Result().StatusCode, body)
	}
}

func TestLoginFailure(t *testing.T) {
	reader := strings.NewReader(`{ "username": "kpors", "password": "incorrectpasswordhere"}`)
	req := httptest.NewRequest(http.MethodGet, "/v1/login", reader)
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	f := ApiLogin(&testRepo)
	f.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status %d, but was %d", http.StatusForbidden, result.StatusCode)
	}
}

func TestLoginOk(t *testing.T) {
	response := login(t)
	t.Logf("JWT: %s", response.Jwt)
}

func TestGetCv(t *testing.T) {
	r := createRouter(&testRepo)

	response := login(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/cv", nil)

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+response.Jwt)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()

	var balls CurriculumVitaeDocument
	mustUnmarshal(t, w.Result().Body, &balls)
}

// logins using the API with a correct username and password against the
// database, then returns a LoginResponse with a JWT to use in subsequent
// authenticated tests.
func login(t *testing.T) LoginResponse {
	reader := strings.NewReader(`{ "username": "kpors", "password": "test"}`)
	req := httptest.NewRequest(http.MethodGet, "/v1/login", reader)
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	f := ApiLogin(&testRepo)
	f.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected to login with the credentials!")
	}

	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Errorf("Unable to read body: %s", err)
	}

	var loginResponse LoginResponse
	if err := json.Unmarshal(body, &loginResponse); err != nil {
		t.Errorf("Unable to unmarshal to LoginResponse")
	}

	return loginResponse
}
