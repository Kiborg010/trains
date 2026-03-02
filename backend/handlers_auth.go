package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{
			OK:    false,
			Error: "invalid json",
		})
		return
	}

	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	if email == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{
			OK:    false,
			Error: "email and password are required",
		})
		return
	}

	if len(password) < 6 {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{
			OK:    false,
			Error: "password must be at least 6 characters",
		})
		return
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, RegisterResponse{
			OK:    false,
			Error: "failed to process password",
		})
		return
	}

	user, err := appStore.CreateUser(email, passwordHash)
	if err != nil {
		if err == ErrUserExists {
			writeJSON(w, http.StatusConflict, RegisterResponse{
				OK:    false,
				Error: "user with this email already exists",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, RegisterResponse{
			OK:    false,
			Error: "failed to create user",
		})
		return
	}

	writeJSON(w, http.StatusCreated, RegisterResponse{
		OK:   true,
		User: user,
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, AuthResponse{
			OK:    false,
			Error: "invalid json",
		})
		return
	}

	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	if email == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, AuthResponse{
			OK:    false,
			Error: "email and password are required",
		})
		return
	}

	user, err := appStore.GetUserByEmail(email)
	if err != nil {
		if err == ErrUserNotFound {
			writeJSON(w, http.StatusUnauthorized, AuthResponse{
				OK:    false,
				Error: "invalid credentials",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, AuthResponse{
			OK:    false,
			Error: "failed to authenticate",
		})
		return
	}

	if !VerifyPassword(user.PasswordHash, password) {
		writeJSON(w, http.StatusUnauthorized, AuthResponse{
			OK:    false,
			Error: "invalid credentials",
		})
		return
	}

	token, err := GenerateJWT(user.ID, user.Email, jwtSecret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, AuthResponse{
			OK:    false,
			Error: "failed to generate token",
		})
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		OK:    true,
		Token: token,
		User:  user,
	})
}

// meHandler returns the current user info (requires valid JWT)
func meHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"ok":    "false",
			"error": "unauthorized",
		})
		return
	}

	user, err := appStore.GetUserByID(userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"ok":    "false",
			"error": "user not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": user,
	})
}
