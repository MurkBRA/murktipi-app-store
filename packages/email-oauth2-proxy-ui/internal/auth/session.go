package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const cookieName = "eop_ui_session"

// Manager handles optional username/password session cookies for the Web UI.
type Manager struct {
	username string
	password string
	secret   []byte
	ttl      time.Duration
}

// NewManager creates an auth Manager. If password is empty, auth is disabled.
func NewManager(username, password string) *Manager {
	if username == "" {
		username = "admin"
	}
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return &Manager{
		username: username,
		password: password,
		secret:   secret,
		ttl:      7 * 24 * time.Hour,
	}
}

// Enabled reports whether login protection is active.
func (m *Manager) Enabled() bool {
	return m.password != ""
}

// ValidateCredentials verifies username and password in constant time.
func (m *Manager) ValidateCredentials(user, pass string) bool {
	if !m.Enabled() {
		return true
	}
	uOK := subtle.ConstantTimeCompare([]byte(user), []byte(m.username)) == 1
	pOK := subtle.ConstantTimeCompare([]byte(pass), []byte(m.password)) == 1
	return uOK && pOK
}

// SetSession writes a signed HttpOnly session cookie.
func (m *Manager) SetSession(w http.ResponseWriter, r *http.Request, user string) {
	exp := time.Now().Add(m.ttl).Unix()
	payload := fmt.Sprintf("%s|%d", user, exp)
	sig := m.sign(payload)
	token := base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sig))

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		MaxAge:   int(m.ttl.Seconds()),
	})
}

// ClearSession deletes the session cookie.
func (m *Manager) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// IsAuthenticated checks whether the request has a valid session cookie (or auth is disabled).
func (m *Manager) IsAuthenticated(r *http.Request) bool {
	if !m.Enabled() {
		return true
	}
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + "|" + parts[1]
	expectedSig := m.sign(payload)
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expectedSig)) != 1 {
		return false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return true
}

// Require wraps an http.HandlerFunc and returns 401 JSON when unauthenticated.
func (m *Manager) Require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.IsAuthenticated(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"authentication required"}`))
			return
		}
		next(w, r)
	}
}

func (m *Manager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
