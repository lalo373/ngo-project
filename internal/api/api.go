// Package api wires the HTTP routes: member logins, payout views,
// admin endpoints and the static SPA.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"payroll/internal/config"
	"payroll/internal/payout"
)

type API struct {
	DB  *sql.DB
	Cfg *config.Config
	Eng *payout.Engine
}

func New(database *sql.DB, cfg *config.Config, eng *payout.Engine) http.Handler {
	a := &API{DB: database, Cfg: cfg, Eng: eng}
	mux := http.NewServeMux()

	// member logins
	mux.HandleFunc("POST /login/executive", a.loginMember("executive"))
	mux.HandleFunc("POST /login/supervisor", a.loginMember("supervisor"))
	mux.HandleFunc("POST /login/district_coordinator", a.loginMember("district_coordinator"))
	mux.HandleFunc("POST /login/regional_coordinator", a.loginMember("regional_coordinator"))
	mux.HandleFunc("POST /admin/login", a.loginAdmin)
	mux.HandleFunc("POST /logout", a.logout)
	mux.HandleFunc("GET /me", a.me)

	// member payout views (any of the four member roles)
	member := a.requireMember()
	mux.Handle("GET /dailyPayout", member(http.HandlerFunc(a.dailyPayout)))
	mux.Handle("GET /weeklyPayouts", member(http.HandlerFunc(a.weeklyPayouts)))
	mux.Handle("GET /calendar", member(http.HandlerFunc(a.calendar)))

	// admin
	admin := a.requireAdmin()
	mux.Handle("GET /admin/dashboard/getDataByDate", admin(http.HandlerFunc(a.reportCSV)))
	mux.Handle("POST /admin/runPayout", admin(http.HandlerFunc(a.runPayout)))
	mux.Handle("GET /admin/members", admin(http.HandlerFunc(a.listMembers)))
	mux.Handle("POST /admin/members", admin(http.HandlerFunc(a.createMember)))
	mux.Handle("PUT /admin/members/{role}/{id}", admin(http.HandlerFunc(a.updateMember)))
	mux.Handle("PUT /admin/members/{role}/{id}/password", admin(http.HandlerFunc(a.resetPassword)))
	mux.Handle("DELETE /admin/members/{role}/{id}", admin(http.HandlerFunc(a.deleteMember)))

	// static SPA (built frontend in ./static) with index.html fallback
	mux.Handle("/", a.staticHandler())

	return logMiddleware(mux)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

// ---- helpers -----------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type loginBody struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ---- member context ----------------------------------------------------

type memberKey struct{}

// memberIdent is the authenticated member attached to a request.
type memberIdent struct {
	Role string
	ID   int
}

func withMember(ctx context.Context, role string, id int) context.Context {
	return context.WithValue(ctx, memberKey{}, memberIdent{Role: role, ID: id})
}

func currentMember(r *http.Request) memberIdent {
	return r.Context().Value(memberKey{}).(memberIdent)
}

// ---- middleware --------------------------------------------------------

func (a *API) requireMember() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				fail(w, 403, "use member login")
				return
			}
			id, err := atoi(s.LoginID)
			if err != nil {
				fail(w, 401, "bad session")
				return
			}
			next.ServeHTTP(w, r.WithContext(withMember(r.Context(), s.Role, id)))
		})
	}
}

func (a *API) requireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, err := a.session(r)
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
			if s == nil {
				fail(w, 401, "not logged in")
				return
			}
			if s.Role != "admin" {
				fail(w, 403, "admin only")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
