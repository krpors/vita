package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt/v5"
)

type ContextKey string

const (
	ContextKeyClaims ContextKey = "claims"
)

type ErrorCode string

const (
	ApiErrorCodeInvalid              ErrorCode = "BadRequest"
	ApiErrorCodeForbidden            ErrorCode = "Forbidden"
	ApiErrorCodeMethodNotAllowed     ErrorCode = "MethodNotAllowed"
	ApiErrorCodeNotFound             ErrorCode = "URINotFound"
	ApiErrorCodeResourceNotFound     ErrorCode = "ResourceNotFound"
	ApiErrorCodeInternalError        ErrorCode = "InternalServerError"
	ApiErrorCodePDFGenerationFailure ErrorCode = "PDFGenerationFailure"
)

var apiErrorCodeMap = map[ErrorCode]int{
	ApiErrorCodeInvalid:              http.StatusBadRequest,
	ApiErrorCodeForbidden:            http.StatusForbidden,
	ApiErrorCodeMethodNotAllowed:     http.StatusMethodNotAllowed,
	ApiErrorCodeNotFound:             http.StatusNotFound,
	ApiErrorCodeResourceNotFound:     http.StatusNotFound,
	ApiErrorCodeInternalError:        http.StatusInternalServerError,
	ApiErrorCodePDFGenerationFailure: http.StatusInternalServerError,
}

type ApiErrorResponse struct {
	Error ApiError `json:"error"`
}

type ApiError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *ApiErrorResponse) Render(w http.ResponseWriter, r *http.Request) error {
	httpCode := http.StatusInternalServerError

	if val, ok := apiErrorCodeMap[e.Error.Code]; ok {
		httpCode = val
	} else {
		log.Printf("WARNING: no mapping found for '%s', using '%d'", e.Error.Code, http.StatusInternalServerError)
		httpCode = http.StatusInternalServerError
	}

	render.Status(r, httpCode)

	return nil
}

func NewApiErrorResponse(code ErrorCode, message string, params ...any) ApiErrorResponse {
	return ApiErrorResponse{
		Error: ApiError{
			Code:    code,
			Message: fmt.Sprintf(message, params...),
		},
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (req *LoginRequest) Bind(r *http.Request) error {
	return nil
}

type LoginResponse struct {
	Jwt string `json:"jwt"`
}

// This is a Chi middleware function to authenticate a request by parsing
// and validating a Bearer token (JWT). Only if the token can be properly
// read and validated, the middleware allows further processing.
//
// After parsing/validating, the token's claims are set in the request context
// using the `ContextKeyClaims` key.
func JWTAuthMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Attempting to authenticate")

		authHeader := r.Header.Get("authorization")
		token, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found {
			apiError := NewApiErrorResponse(ApiErrorCodeForbidden, "No Bearer token present in Authorization header")
			render.Render(w, r, &apiError)
			return
		}

		// log.Printf("Token is %s", token)

		parsedToken, err := jwt.ParseWithClaims(token, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("secretsecret"), nil
		})

		// TODO: verify token expiry
		// TODO: configure or finalize secret

		if err != nil {
			log.Printf("Unable to validate token: %s", err)
			apiError := NewApiErrorResponse(ApiErrorCodeForbidden, "Given token could not be validated")
			render.Render(w, r, &apiError)
			return
		}

		claims := parsedToken.Claims.(*CustomClaims)
		log.Printf("Successfully authenticated with JWT for user %s", claims.Subject)
		r = r.WithContext(context.WithValue(r.Context(), ContextKeyClaims, claims))

		// Successfully authenticated, call the next handler.
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// The claims returned bu ApiLogin.
type CustomClaims struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	jwt.RegisteredClaims
}

// NewCustomClaims create Vita custom claims based on the authorized user of
// the API.
func NewCustomClaims(user User) CustomClaims {
	claims := CustomClaims{
		user.Username,
		user.Profile.FirstName,
		user.Profile.LastName,
		jwt.RegisteredClaims{
			Issuer:    "vita-go",
			Subject:   user.Id.Hex(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24 * 7)), // week
		},
	}

	return claims
}

// Returns a sort function to sort the CurriculumVitaeDocument based on the version,
// in descending order (so the most recent version comes first in the results).
func SortByVersion() func(left, right CurriculumVitaeDocument) int {
	return func(left, right CurriculumVitaeDocument) int {
		if left.Metadata.Version == right.Metadata.Version {
			return 0
		}

		return left.Metadata.Version - right.Metadata.Version
	}
}

// =============================================================================
// API REST handler functions
// =============================================================================

func ApiCreateUser(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request CreateUserRequest
		err := render.Bind(r, &request)

		if err != nil {
			apiResponse := NewApiErrorResponse(ApiErrorCodeInvalid, "unable to read request")
			render.Render(w, r, &apiResponse)
			return
		}

		user := User{
			Username: request.Username,
			Password: request.Password,
		}
		_, err = repo.CreateUser(r.Context(), user)
		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInvalid, "an account with that username already exists")
			render.Render(w, r, &apiError)
			return
		}
	}
}

// API call to login to the system using the MongoDB backend.
func ApiLogin(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// form-stuff:
		// err := r.ParseForm()
		// if err != nil {
		// 	apiError := NewApiErrorResponse(ApiErrorCodeInvalid, "Could not parse the form request")
		// 	render.Render(w, r, &apiError)
		// 	return
		// }

		// user := r.FormValue("username")
		// pass := r.FormValue("password")
		loginRequest := &LoginRequest{}
		if err := render.Bind(r, loginRequest); err != nil {
			log.Printf("Error occurred while trying to bind: %s", err)
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Could not bind login properties!")
			render.Render(w, r, &apiError)
			return
		}

		log.Printf("Authentication attempt for username '%s'", loginRequest.Username)
		mongoUser, found, err := repo.Authenticate(r.Context(), loginRequest.Username, loginRequest.Password)

		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Unable to authenticate due to an internal server error")
			render.Render(w, r, &apiError)
			return
		}

		if !found {
			apiError := NewApiErrorResponse(ApiErrorCodeForbidden, "Unauthorized to login with the given credentials")
			render.Render(w, r, &apiError)
			return
		}

		// User is found
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, NewCustomClaims(mongoUser))
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		tokenString, err := token.SignedString([]byte("secretsecret"))
		if err != nil {
			log.Printf("Could not sign token? %s", err)
		}

		resp := LoginResponse{
			Jwt: tokenString,
		}

		val, _ := json.Marshal(resp)
		w.Write(val)
	}
}

// runCommand runs the typst command with a template, using a serialized
// CurriculumVitae JSON structure as input. When the generation succeeded,
// the result of the stdout is returned as bytes (which will contain PDF content).
// In case anything fails while running the command or connecting the stdout or
// stderr pipes, error is non-nil and the byte slice will remain nil.
func runCommand(cv *CurriculumVitae) ([]byte, error) {
	startOfGeneration := time.Now()
	log.Printf("Executing PDF generation for CV...")

	bytes, _ := json.Marshal(cv)

	cmd := exec.Command(
		"typst",                       // the command
		"compile",                     // compile only
		"testdata/example.typ",        // template to use
		"--input",                     // specify input data
		fmt.Sprintf("data=%s", bytes), // 'data' is the key, serialized JSON is value
		"-")                           // output the stdout

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Could not connect to stdout: %s", err)
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Could not connect to stderr: %s", err)
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Could not start command: %s", err)
		return nil, err
	}

	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		log.Printf("WAT! %s", err)
		return nil, err
	}

	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		log.Printf("Could not read from stderr: %s", err)
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		exitError := err.(*exec.ExitError)
		log.Printf("Command exited with statuscode %d. Error message is: %s", exitError.ExitCode(), stderrBytes)
		return nil, err
	}

	log.Printf("Generation took %d ms", time.Since(startOfGeneration).Milliseconds())

	return stdoutBytes, nil
}

func writeCvAsPDF(w http.ResponseWriter, r *http.Request, cv *CurriculumVitae) {
	result, err := runCommand(cv)
	if err != nil {
		log.Printf("Unable to generate CV as PDF: %s", err)
		apiError := NewApiErrorResponse(ApiErrorCodePDFGenerationFailure, "The PDF generation failed. Check the server logs!")
		render.Render(w, r, &apiError)
		return
	}

	w.Header().Add("content-type", "application/pdf")
	w.Write(result)
}

// Get user CV data, using the JWT provided. This call should be protected by JWT
// middleware.
func ApiGetUserCvData(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(ContextKeyClaims).(*CustomClaims)
		cv, found, err := repo.GetCurrentCV(r.Context(), claims.Subject)

		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Shit: %s", err)
			render.Render(w, r, &apiError)
			return
		}

		accept := r.Header.Get("accept")

		if !found {
			apiError := NewApiErrorResponse(ApiErrorCodeResourceNotFound, "The current user does not have a persisted CV (yet)")
			render.Render(w, r, &apiError)
			return
		}

		if accept == "application/pdf" {
			writeCvAsPDF(w, r, &cv.Cv)
			return
		} else {
			b, _ := json.Marshal(cv)
			w.Write(b)
		}
	}
}

func ApiPostUserCvData(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(ContextKeyClaims).(*CustomClaims)

		b, err := io.ReadAll(r.Body)
		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Unable to read HTTP request body")
			render.Render(w, r, &apiError)
			return
		}
		var cv CurriculumVitae
		if err = json.Unmarshal(b, &cv); err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInvalid, "Unable to deserialize JSON properly: %s", err)
			render.Render(w, r, &apiError)
			return
		}

		if err = repo.SaveNewCV(r.Context(), claims.Subject, &cv); err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Unable to persist CV entry")
			render.Render(w, r, &apiError)
			return
		}
	}

}

// ApiGetRevisions gets the CV revisions a user may have.
func ApiGetRevisions(repo *MongoRepository) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(ContextKeyClaims).(*CustomClaims)
		revisions, err := repo.FindRevisionsForUser(r.Context(), claims.Subject)
		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Could not fetch revisions")
			render.Render(w, r, &apiError)
			return
		}

		slices.SortFunc(revisions, SortByVersion())

		b, err := json.Marshal(revisions)
		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Could not render revisions")
			render.Render(w, r, &apiError)
			return
		}

		w.Write(b)
	}
}

func ApiPostPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Creating a preview document")

		var previewReq PreviewRequest
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &previewReq); err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInvalid, "Could not parse JSON: %s", err)
			render.Render(w, r, &apiError)
			return
		}

		log.Printf("Template specified: %s", previewReq.Template)

		writeCvAsPDF(w, r, &previewReq.Cv)
	}
}

func ApiDeleteAllRevisions(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(ContextKeyClaims).(*CustomClaims)

		log.Printf("Attempting to delete ALL revisions for user %s", claims.Subject)

		deleted, err := repo.DeleteAllRevisions(r.Context(), claims.Subject)
		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Unable to delete revision due to a internal error")
			render.Render(w, r, &apiError)
			return
		}

		log.Printf("Deleted all %d revisions for user %s", deleted, claims.Subject)
	}
}

func ApiDeleteSingleRevision(repo *MongoRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(ContextKeyClaims).(*CustomClaims)

		versionToDelete := chi.URLParam(r, "version")
		version, err := strconv.Atoi(versionToDelete)
		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInvalid, "Could not use path variable '%s' as a version integer. Please specify a version number.", versionToDelete)
			render.Render(w, r, &apiError)
			return
		}

		log.Printf("Attempting to delete revision %s for user %s", versionToDelete, claims.Subject)

		deleted, err := repo.DeleteSingleRevision(r.Context(), claims.Subject, version)
		if err != nil {
			apiError := NewApiErrorResponse(ApiErrorCodeInternalError, "Unable to delete revision due to a internal error")
			render.Render(w, r, &apiError)
			return
		}

		if deleted {
			log.Printf("Deleted revision!")
		} else {
			log.Printf("No revision with version %s for user %s could be found to delete", versionToDelete, claims.Subject)
			apiError := NewApiErrorResponse(ApiErrorCodeResourceNotFound, "No revision with version %s could be found to delete", versionToDelete)
			render.Render(w, r, &apiError)
			return
		}
	}
}
