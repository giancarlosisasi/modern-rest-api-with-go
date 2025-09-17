package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"shopping/config"
	"shopping/database"
	db_queries "shopping/database/queries"
	"shopping/repository"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/mock/gomock"
)

func TestAddCacheHeaders(t *testing.T) {
	app := App{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/lists", nil)
	rec := httptest.NewRecorder()
	handler := app.addCacheHeaders(testHandler)

	handler(rec, req)

	if rec.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Errorf("Not valid Cache-Control found, got %v, want %v", rec.Header().Get("Cache-Control"), "public, max-age=300")
	}

	if rec.Header().Get("Expires") == "" {
		t.Errorf("Not valid Expires, got %v, want not empty", rec.Header().Get("Expires"))
	}
}

func TestHandleLogin(t *testing.T) {

	ctrl := gomock.NewController(t)
	mock := repository.NewMockSessionRepository(ctrl)

	app := App{
		SessionRepository: mock,
	}

	mock.EXPECT().AddSession("admin").Return(
		&db_queries.AddSessionRow{
			ID:       pgtype.UUID{Bytes: [16]byte{'a'}, Valid: true},
			Token:    "test-token",
			Username: "admin",
			ExpiresAt: pgtype.Timestamptz{
				Time:  time.Now(),
				Valid: true,
			},
			CreatedAt: pgtype.Timestamptz{
				Time:  time.Now(),
				Valid: true,
			},
			UpdatedAt: pgtype.Timestamptz{
				Time:  time.Now(),
				Valid: true,
			},
		},
		nil,
	)

	req := httptest.NewRequest("POST", "/v1/login", strings.NewReader(`{"username": "admin","password":"password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.handleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleLogin() status = %v, want %v", rec.Code, http.StatusOK)
	}
}

// integration with "real" database
func TestLoginApi(t *testing.T) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:17",
		postgres.WithInitScripts(filepath.Join("testdata", "session-init-db.sql")),
		postgres.WithDatabase("shoppinglist"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections"),
				// this will automatically search for an available port in case the default (5432) is already in use
				wait.ForListeningPort("5432/tcp"),
			).WithDeadline(30*time.Second),
		),
	)

	if err != nil {
		t.Fatalf("failed to start the container: %s", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Fatalf("failed to terminate pgContainer: %s", err)
		}
	})

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	assert.NoError(t, err)

	config := config.Config{
		DBUrl: connStr,
	}

	dbpool, err := database.NewDB(&config)
	if err != nil {
		t.Fatalf("cannot connect to db: %s", err)
	}

	dbQueries := db_queries.New(dbpool)

	sessionRepo := repository.NewSessionRepository(dbQueries)

	app := App{
		SessionRepository: sessionRepo,
		DBQueries:         dbQueries,
	}

	req := httptest.NewRequest("POST", "/v1/login", strings.NewReader(`{"username":"admin","password":"password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.handleLogin(rec, req)

	assert.Equal(t, rec.Code, http.StatusOK, "handleLogin response is not ok")
}

// =========== E2E testing ===============

func makeRequest(method, url string, body io.Reader, token string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return http.DefaultClient.Do(req)
}

func TestLoginAndList(t *testing.T) {
	baseURL := "http://localhost:8080"
	reqBody := strings.NewReader(`{"username":"admin","password":"password"}`)

	resp, err := makeRequest("POST", baseURL+"/v1/login", reqBody, "")
	if err != nil {
		t.Fatalf("Failed to make a request: %s", err)
	}

	defer resp.Body.Close()

	var response map[string]string
	json.NewDecoder(resp.Body).Decode(&response)

	token := response["token"]
	if token == "" {
		t.Fatal("No token returned from login")
	}

	id, _ := rand.Int(rand.Reader, big.NewInt(100))
	listName := fmt.Sprintf("Test list %d", id)

	reqBody = strings.NewReader(fmt.Sprintf(`{"name":"%s", "items": []}`, listName))
	resp2, err := makeRequest("POST", baseURL+"/v1/lists", reqBody, token)
	if err != nil {
		t.Fatalf("failed to make a request: %s", err)
	}

	defer resp2.Body.Close()

	assert.Equal(t, resp2.StatusCode, http.StatusCreated, "error to create a list")

	resp3, err := makeRequest("GET", baseURL+"/v1/lists", nil, token)
	if err != nil {
		t.Fatalf("resp3: failed to make a request: %s", err)
	}

	var lists []db_queries.ShoppingList
	json.NewDecoder(resp3.Body).Decode(&lists)
	found := false
	for _, list := range lists {
		if list.Name == listName {
			found = true
		}
	}

	assert.Equal(t, found, true, fmt.Sprintf("unable to find list with name '%s'", listName))
}
