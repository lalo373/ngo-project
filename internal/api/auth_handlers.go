package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"payroll/internal/auth"
)

func (a *API) session(r *http.Request) (*auth.Session, error) {
	return auth.GetSession(a.DB, r)
}

func atoi(s string) (int, error) { return strconv.Atoi(s) }

// ---- login / session handlers ------------------------------------------

func (a *API) loginMember(role string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b loginBody
		if err := decode(r, &b); err != nil || b.ID == 0 || b.Password == "" {
			fail(w, 400, "id and password required")
			return
		}
		token, err := auth.LoginMember(a.DB, role, b.ID, b.Password)
		if errors.Is(err, auth.ErrInvalid) {
			fail(w, 401, "invalid id or password")
			return
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		auth.SetCookie(w, token)
		name, _ := memberName(a.DB, role, b.ID)
		writeJSON(w, 200, map[string]any{"role": role, "id": b.ID, "name": name})
	}
}

func (a *API) loginAdmin(w http.ResponseWriter, r *http.Request) {
	var b loginBody
	if err := decode(r, &b); err != nil || b.Username == "" || b.Password == "" {
		fail(w, 400, "username and password required")
		return
	}
	token, err := auth.LoginAdmin(a.DB, b.Username, b.Password)
	if errors.Is(err, auth.ErrInvalid) {
		fail(w, 401, "invalid username or password")
		return
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	auth.SetCookie(w, token)
	writeJSON(w, 200, map[string]any{"role": "admin", "id": 0, "name": "Admin"})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	auth.Logout(a.DB, w, r)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	s, err := a.session(r)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if s == nil {
		fail(w, 401, "not logged in")
		return
	}
	if s.Role == "admin" {
		writeJSON(w, 200, map[string]any{"role": "admin", "id": 0, "name": "Admin"})
		return
	}
	id, _ := strconv.Atoi(s.LoginID)
	name, _ := memberName(a.DB, s.Role, id)
	writeJSON(w, 200, map[string]any{"role": s.Role, "id": id, "name": name})
}

// ---- shared helpers ----------------------------------------------------

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return jsonDecode(r, v)
}

// memberName looks up a member's display name by role and id.
func memberName(db *sql.DB, role string, id int) (string, error) {
	query, args, err := memberLookup(role, id)
	if err != nil {
		return "", err
	}
	var name string
	err = db.QueryRow(query, args...).Scan(&name)
	return name, err
}

// ---- today in IST ------------------------------------------------------

func todayIST() time.Time {
	n := time.Now().In(auth.IST)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, auth.IST)
}

// weekStart returns the Monday 00:00 IST of the week containing t.
func weekStart(t time.Time) time.Time {
	t = t.In(auth.IST)
	offset := (int(t.Weekday()) + 6) % 7 // Mon=0 ... Sun=6
	d := t.AddDate(0, 0, -offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, auth.IST)
}

// weekEnd returns the Sunday 23:59:59 IST of the week containing t.
func weekEnd(t time.Time) time.Time {
	return weekStart(t).AddDate(0, 0, 6)
}
