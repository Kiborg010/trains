package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupAuthTestStore() {
	appStore = NewInMemoryStore()
	jwtSecret = "test-secret-key"
}

func TestRegisterHandler(t *testing.T) {
	setupAuthTestStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/register", registerHandler)

	payload := RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 201 or 400, got %d", w.Code)
	}

	var resp RegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if w.Code == http.StatusCreated && resp.OK {
		if resp.User == nil || resp.User.Email != "test@example.com" {
			t.Fatal("expected user to be created")
		}
	}
}

func TestLoginHandler(t *testing.T) {
	setupAuthTestStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", loginHandler)

	payload := LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Logf("expected status 401 for nonexistent user, got %d", w.Code)
	}

	var resp AuthResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.OK {
		t.Fatal("expected login to fail for nonexistent user")
	}
}

func TestHashPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !VerifyPassword(hash, password) {
		t.Fatal("password verification failed")
	}

	if VerifyPassword(hash, "wrongpassword") {
		t.Fatal("password verification should fail with wrong password")
	}
}

func TestJWT(t *testing.T) {
	secret := "test-secret-key"
	userID := 123
	email := "test@example.com"

	token, err := GenerateJWT(userID, email, secret)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	if token == "" {
		t.Fatal("generated token is empty")
	}

	claims, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("failed to verify JWT: %v", err)
	}

	if claims.UserID != userID || claims.Email != email {
		t.Fatal("JWT claims do not match")
	}

	_, err = VerifyJWT(token, "wrong-secret")
	if err == nil {
		t.Fatal("expected JWT verification to fail with wrong secret")
	}
}
