package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/bunnydb/bunnydb/flow/shared"
)

// ============================================================================
// Types
// ============================================================================

type contextKey string

const userClaimsKey contextKey = "userClaims"

type UserClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
	Role     string `json:"role"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type UserResponse struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// ============================================================================
// Middleware
// ============================================================================

func (h *Handler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		tokenString := parts[1]
		claims := &UserClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(h.Config.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userClaimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return h.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(userClaimsKey).(*UserClaims)
		if claims.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	})
}

// Convenience wrappers for route registration
func (h *Handler) open(next http.HandlerFunc) http.HandlerFunc {
	return corsMiddleware(next)
}

func (h *Handler) authed(next http.HandlerFunc) http.HandlerFunc {
	return corsMiddleware(h.authMiddleware(next))
}

func (h *Handler) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return corsMiddleware(h.adminMiddleware(next))
}

// ============================================================================
// Auth Handlers
// ============================================================================

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	var passwordHash string
	var role string
	err := h.CatalogPool.QueryRow(r.Context(),
		"SELECT password_hash, role FROM bunny_internal.users WHERE username = $1",
		req.Username,
	).Scan(&passwordHash, &role)

	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Issue JWT
	claims := &UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   req.Username,
		},
		Username: req.Username,
		Role:     role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.Config.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:    tokenString,
		Username: req.Username,
		Role:     role,
	})
}

func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(userClaimsKey).(*UserClaims)
	writeJSON(w, http.StatusOK, map[string]string{
		"username": claims.Username,
		"role":     claims.Role,
	})
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.CatalogPool.Query(r.Context(),
		"SELECT id, username, role, created_at FROM bunny_internal.users ORDER BY created_at")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	defer rows.Close()

	var users []UserResponse
	for rows.Next() {
		var u UserResponse
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &createdAt); err != nil {
			continue
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		users = append(users, u)
	}

	if users == nil {
		users = []UserResponse{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	if req.Role == "" {
		req.Role = "readonly"
	}
	if req.Role != "admin" && req.Role != "readonly" {
		writeError(w, http.StatusBadRequest, "role must be 'admin' or 'readonly'")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	_, err = h.CatalogPool.Exec(r.Context(),
		"INSERT INTO bunny_internal.users (username, password_hash, role) VALUES ($1, $2, $3)",
		req.Username, string(hash), req.Role,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"username": req.Username,
		"role":     req.Role,
	})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username required")
		return
	}

	// Cannot delete yourself
	claims := r.Context().Value(userClaimsKey).(*UserClaims)
	if claims.Username == username {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	result, err := h.CatalogPool.Exec(r.Context(),
		"DELETE FROM bunny_internal.users WHERE username = $1", username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"deleted": username})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(userClaimsKey).(*UserClaims)

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password required")
		return
	}

	// Verify current password
	var currentHash string
	err := h.CatalogPool.QueryRow(r.Context(),
		"SELECT password_hash FROM bunny_internal.users WHERE username = $1",
		claims.Username,
	).Scan(&currentHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.CurrentPassword)); err != nil {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	// Update password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	_, err = h.CatalogPool.Exec(r.Context(),
		"UPDATE bunny_internal.users SET password_hash = $1 WHERE username = $2",
		string(newHash), claims.Username,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

// ============================================================================
// Startup Functions
// ============================================================================

func EnsureUsersTable(pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS bunny_internal.users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role VARCHAR(20) NOT NULL DEFAULT 'readonly',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		slog.Error("failed to ensure users table", slog.Any("error", err))
	}
}

func SeedAdmin(pool *pgxpool.Pool, config *shared.Config) {
	if config.AdminPassword == "" {
		slog.Warn("BUNNY_ADMIN_PASSWORD not set, skipping admin seed")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Only seed if no users exist
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM bunny_internal.users").Scan(&count)
	if err != nil {
		slog.Error("failed to check user count", slog.Any("error", err))
		return
	}

	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(config.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("failed to hash admin password", slog.Any("error", err))
		return
	}

	_, err = pool.Exec(ctx,
		"INSERT INTO bunny_internal.users (username, password_hash, role) VALUES ($1, $2, 'admin')",
		config.AdminUser, string(hash),
	)
	if err != nil {
		slog.Error("failed to seed admin user", slog.Any("error", err))
		return
	}

	slog.Info("seeded default admin user", slog.String("username", config.AdminUser))
}
