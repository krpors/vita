package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	testMongoClient *mongo.Client
	testRepo        MongoRepository
	testApiResource *VitaJsonAPIResource
	testRouter      chi.Router
)

func mustUnmarshal(t *testing.T, response string, v any) {
	err := json.Unmarshal([]byte(response), v)
	require.Nilf(t, err, "unmarshalling must succeed without error")
}

func mustReadFile(file string) string {
	f, err := os.Open("testdata/entry.json")
	if err != nil {
		panic("can't read " + file + " due to " + err.Error())
	}
	defer f.Close()

	s := mustRead(f)
	return s
}

func mustRead(rc io.ReadCloser) string {
	bytes, err := io.ReadAll(rc)
	if err != nil {
		panic("could not read body")
	}

	return string(bytes)
}

// setupTestUser actually creates a test user in the database. The function
// returns a function, which can be deferred so that the test user is deleted
// after running a test which requires a user.
//
// This ascertains that each test is run with a clean account, with zero assigned
// 'foreign keys'.
func setupTestUser(t *testing.T) func() {
	objectId, err := testRepo.CreateUser(context.TODO(), User{
		Username: "testuser",
		Password: "testuser",
	})

	require.Nilf(t, err, "could not create a testing user, see error")

	return func() {
		testRepo.DeleteUser(context.TODO(), objectId.Hex())
	}
}

func createTestingMongoClient() *mongo.Client {
	uri := "mongodb://localhost:27017"
	opts := options.Client().ApplyURI(uri)

	client, err := mongo.Connect(opts)
	if err != nil {
		panic("Unable to create a connection using the supplied string")
	}

	bleh, fc := context.WithTimeout(context.Background(), time.Second*2)
	if err := client.Ping(bleh, nil); err != nil {
		panic("Could not create a connection to the local Mongo database at :27017")
	}
	defer fc()

	return client
}

// logins using the API with a correct username and password against the
// database, then returns a LoginResponse with a JWT to use in subsequent
// authenticated tests.
func login(t *testing.T) LoginResponse {
	reader := strings.NewReader(`{ "username": "testuser", "password": "testuser"}`)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/login", reader)
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	testApiResource.ApiLogin(w, req)
	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected to login with the credentials!")
	}

	var loginResponse LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &loginResponse); err != nil {
		t.Errorf("Unable to unmarshal to LoginResponse")
	}

	return loginResponse
}

func TestMain(t *testing.M) {
	testMongoClient = createTestingMongoClient()
	defer testMongoClient.Disconnect(context.TODO())
	testRepo = NewMongoRepository(testMongoClient)
	testRouter = createRouter(&testRepo)
	testApiResource = NewVitaJsonAPIResource(&testRepo)
	code := t.Run()
	os.Exit(code)
}

func TestMethodNotAllowed(t *testing.T) {
	// GET is not allowed on /api/v1/user
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
	w := httptest.NewRecorder()

	testRouter.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()

	assert.Equal(t, 405, w.Result().StatusCode)

	var errResponse ApiErrorResponse
	mustUnmarshal(t, w.Body.String(), &errResponse)

	assert.Equal(t, ApiErrorCodeMethodNotAllowed, errResponse.Error.Code)
}

func TestLoginFailure(t *testing.T) {
	reader := strings.NewReader(`{ "username": "kpors", "password": "incorrectpasswordhere"}`)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/login", reader)
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	testApiResource.ApiLogin(w, req)
	result := w.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status %d, but was %d", http.StatusForbidden, result.StatusCode)
	}
}

func TestLoginEmptyUsernamePassword(t *testing.T) {
	reader := strings.NewReader(`{ "username": "", "password": ""}`)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/login", reader)
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	testApiResource.ApiLogin(w, req)
	result := w.Result()
	defer result.Body.Close()

	assert.Equal(t, result.StatusCode, http.StatusBadRequest)
	// t.Log(w.Body.String())
	var errResponse ApiErrorResponse
	mustUnmarshal(t, w.Body.String(), &errResponse)
	assert.Equal(t, ApiErrorCodeValidationFailed, errResponse.Error.Code)
	t.Log(errResponse.Error.Message)
}

func TestLoginOk(t *testing.T) {
	defer setupTestUser(t)()
	response := login(t)
	t.Logf("JWT: %s", response.Jwt)
}

// Tests the creation of a CV, getting it as JSON and getting it as PDF.
// This test obviously requires typst to be available on the PATH.
func TestCreateAndGet(t *testing.T) {
	defer setupTestUser(t)()

	// 1: login
	loginResponse := login(t)

	// 2: create cv
	body := mustReadFile("./testdata/entry.json")
	bleh := strings.NewReader(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cv", bleh)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+loginResponse.Jwt)

	w := httptest.NewRecorder()

	testRouter.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Result().StatusCode)

	// 3: get cv as JSON
	req = httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+loginResponse.Jwt)

	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Result().StatusCode)

	var cv CurriculumVitaeDocument
	mustUnmarshal(t, w.Body.String(), &cv)

	// some sanity checks here
	assert.Equal(t, 1, cv.Metadata.Version)
	assert.Equal(t, "John", cv.Cv.FirstName)
	assert.Equal(t, "Doe", cv.Cv.LastName)
	assert.Equal(t, 2, len(cv.Cv.Links))

	// 4: get cv as PDF
	req = httptest.NewRequest(http.MethodGet, "/api/v1/cv", nil)
	req.Header.Add("Accept", "application/pdf")
	req.Header.Add("Authorization", "Bearer "+loginResponse.Jwt)

	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	f, err := os.Create("example.pdf")
	if err != nil {
		assert.FailNow(t, "nil")
	}
	w.Body.WriteTo(f)

	// pdfHeader := w.Body.Next(4)
	// expected := []byte{0x25, 0x50, 0x44, 0x46} // PDF header magic
	// assert.Equal(t, 4, len(pdfHeader))
	// assert.Equal(t, expected, pdfHeader)
	// assert.Equal(t, 200, w.Result().StatusCode)
}

func TestNoRegisteredUri(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/non-existent-uri", nil)
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	var response ApiErrorResponse
	mustUnmarshal(t, w.Body.String(), &response)
	assert.Equal(t, ApiErrorCodeNotFound, response.Error.Code)
}

func TestRequiredAuthenticatedEndpoints(t *testing.T) {
	uris := []struct {
		method string
		uri    string
	}{
		{method: http.MethodGet, uri: "/api/v1/cv"},
		{method: http.MethodPost, uri: "/api/v1/cv"},
		{method: http.MethodGet, uri: "/api/v1/cv/revisions"},
		{method: http.MethodDelete, uri: "/api/v1/cv/revisions"},
		{method: http.MethodPost, uri: "/api/v1/cv/preview"},
	}

	for _, u := range uris {
		name := fmt.Sprintf("%s to %s", u.method, u.uri)
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(u.method, u.uri, nil)
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()
			testRouter.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Result().StatusCode)
		})
	}
}

func TestPreview(t *testing.T) {
	defer setupTestUser(t)()

	// 1: login
	loginResponse := login(t)

	// 2: read CV data from file
	var cvData CurriculumVitae
	body := mustReadFile("./testdata/entry.json")
	mustUnmarshal(t, body, &cvData)

	previewData := PreviewRequest{
		Configuration: PreviewConfiguration{
			Template: "example",
		},
		Cv: cvData,
	}

	bs, err := json.Marshal(previewData)
	require.Nilf(t, err, "must unlkasdjlksa")
	bleh := bytes.NewReader(bs)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cv/preview", bleh)
	req.Header.Add("Authorization", "Bearer "+loginResponse.Jwt)
	req.Header.Add("Content-Type", "application/json")

	w := httptest.NewRecorder()

	testRouter.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Result().StatusCode)

	pdfHeader := w.Body.Next(4)
	expected := []byte{0x25, 0x50, 0x44, 0x46} // PDF header magic
	assert.Equal(t, 4, len(pdfHeader))
	assert.Equal(t, expected, pdfHeader)
	assert.Equal(t, 200, w.Result().StatusCode)
}
