package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// IST is Indian Standard Time (UTC+5:30, no DST).
var IST = time.FixedZone("IST", 5*60*60+30*60)

const (
	// SessionCookie is the session cookie name.
	SessionCookie = "sid"
	// SessionTTL: all sessions are logged out after 24 hours.
	SessionTTL = 24 * time.Hour
)

// Session is the authenticated identity attached to a request.
type Session struct {
	Role    string // executive | supervisor | district_coordinator | regional_coordinator | admin
	LoginID string // member id as text, or admin username
}

// ErrInvalid is returned when credentials do not match.
var ErrInvalid = errInvalid{}

type errInvalid struct{}

func (errInvalid) Error() string { return "invalid credentials" }

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// EnsureAdmin creates the admin account if it does not exist yet.
func EnsureAdmin(db *sql.DB, user, pass string) error {
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM app_user WHERE username=$1)`, user).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	hash, err := HashPassword(pass)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO app_user (username, password_hash) VALUES ($1,$2) ON CONFLICT (username) DO NOTHING`, user, hash)
	return err
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func createSession(db *sql.DB, role, loginID string) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = db.Exec(`INSERT INTO session (token, role, login_id, expires_at) VALUES ($1,$2,$3,$4)`,
		token, role, loginID, time.Now().Add(SessionTTL))
	if err != nil {
		return "", err
	}
	return token, nil
}

// LoginMember checks hashed credentials and opens a 24h session.
// Returns the session token; the caller sets it as a cookie.
func LoginMember(db *sql.DB, role string, id int, password string) (string, error) {
	var hash string
	err := db.QueryRow(`SELECT password_hash FROM credential WHERE role=$1 AND login_id=$2`, role, id).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	if !CheckPassword(hash, password) {
		return "", ErrInvalid
	}
	return createSession(db, role, strconv.Itoa(id))
}

// LoginAdmin checks the admin account and opens a 24h session.
func LoginAdmin(db *sql.DB, username, password string) (string, error) {
	var hash string
	err := db.QueryRow(`SELECT password_hash FROM app_user WHERE username=$1`, username).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", ErrInvalid
	}
	if err != nil {
		return "", err
	}
	if !CheckPassword(hash, password) {
		return "", ErrInvalid
	}
	return createSession(db, "admin", username)
}

// SetCookie writes the session cookie (24h, HttpOnly).
func SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSession resolves the session cookie to a Session, or nil when absent.
// Expired sessions are deleted on sight.
func GetSession(db *sql.DB, r *http.Request) (*Session, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return nil, nil
	}
	var s Session
	var expires time.Time
	err = db.QueryRow(`SELECT role, login_id, expires_at FROM session WHERE token=$1`, c.Value).Scan(&s.Role, &s.LoginID, &expires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(expires) {
		db.Exec(`DELETE FROM session WHERE token=$1`, c.Value)
		return nil, nil
	}
	return &s, nil
}

// Logout deletes the current session and clears the cookie.
func Logout(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		db.Exec(`DELETE FROM session WHERE token=$1`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// PurgeExpiredSessions removes sessions past their 24h lifetime.
func PurgeExpiredSessions(db *sql.DB) {
	db.Exec(`DELETE FROM session WHERE expires_at < now()`)
}
