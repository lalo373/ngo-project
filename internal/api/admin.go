package api

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"payroll/internal/auth"
)

// reportCSV downloads the day's payout report as CSV.
// GET /admin/dashboard/getDataByDate?date=YYYY-MM-DD
func (a *API) reportCSV(w http.ResponseWriter, r *http.Request) {
	dateStr := queryStrings(r, "date", todayIST().Format("2006-01-02"))
	day, err := time.ParseInLocation("2006-01-02", dateStr, auth.IST)
	if err != nil {
		fail(w, 400, "date must be YYYY-MM-DD")
		return
	}

	// names for all members, grouped by role
	names := map[string]map[int]string{
		"executive": {}, "supervisor": {}, "district_coordinator": {}, "regional_coordinator": {},
	}
	for role, ri := range roles {
		func(role string, ri roleInfo) {
			rows, err := a.DB.Query("SELECT " + ri.IDCol + ", name FROM " + ri.Table)
			if err != nil {
				return
			}
			defer rows.Close()
			for rows.Next() {
				var id int
				var name string
				if rows.Scan(&id, &name) == nil {
					names[role][id] = name
				}
			}
		}(role, ri)
	}

	rows, err := a.DB.Query(
		`SELECT role, login_id, eligible, amount, COALESCE(metric,0) FROM payout WHERE payout_date = $1 ORDER BY role, login_id`,
		day,
	)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="payout-report-%s.csv"`, dateStr))
	cw := csv.NewWriter(w)
	cw.Write([]string{"date", "role", "id", "name", "eligible", "amount_inr", "metric"})
	for rows.Next() {
		var role string
		var id int
		var eligible bool
		var amount, metric float64
		if err := rows.Scan(&role, &id, &eligible, &amount, &metric); err != nil {
			continue
		}
		cw.Write([]string{
			dateStr, role, strconv.Itoa(id), names[role][id],
			boolStr(eligible), fmt.Sprintf("%.2f", amount), fmt.Sprintf("%.1f", metric),
		})
	}
	cw.Flush()
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// runPayout triggers the eligibility computation for a date on demand.
// POST /admin/runPayout?date=YYYY-MM-DD
func (a *API) runPayout(w http.ResponseWriter, r *http.Request) {
	dateStr := queryStrings(r, "date", todayIST().Format("2006-01-02"))
	day, err := time.ParseInLocation("2006-01-02", dateStr, auth.IST)
	if err != nil {
		fail(w, 400, "date must be YYYY-MM-DD")
		return
	}
	n, err := a.Eng.ComputeDate(r.Context(), day)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"computed": n, "date": dateStr})
}

// ---- member CRUD -------------------------------------------------------

type memberBody struct {
	Role     string `json:"role"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Area     string `json:"area"`
	ParentID *int   `json:"parent_id"`
	Password string `json:"password"`
}

// listMembers GET /admin/members?role=executive
func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	role := queryStrings(r, "role", "executive")
	ri, ok := roleMeta(role)
	if !ok {
		fail(w, 400, "unknown role")
		return
	}
	sel := ri.IDCol + " AS id, name, phone, area"
	if ri.ParentCol != "" {
		sel += ", " + ri.ParentCol + " AS parent_id"
	} else {
		sel += ", NULL::int AS parent_id"
	}
	rows, err := a.DB.Query("SELECT " + sel + " FROM " + ri.Table + " ORDER BY id")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type member struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		Area     string `json:"area"`
		ParentID *int   `json:"parent_id"`
	}
	members := []member{}
	for rows.Next() {
		var m member
		var phone sql.NullString
		if err := rows.Scan(&m.ID, &m.Name, &phone, &m.Area, &m.ParentID); err != nil {
			fail(w, 500, err.Error())
			return
		}
		m.Phone = nullString(phone)
		members = append(members, m)
	}
	writeJSON(w, 200, map[string]any{"members": members})
}

// createMember POST /admin/members  {role,name,phone,area,parent_id,password}
func (a *API) createMember(w http.ResponseWriter, r *http.Request) {
	var b memberBody
	if err := decode(r, &b); err != nil {
		fail(w, 400, err.Error())
		return
	}
	ri, ok := roleMeta(b.Role)
	if !ok {
		fail(w, 400, "unknown role")
		return
	}
	if b.Name == "" || b.Phone == "" || b.Password == "" {
		fail(w, 400, "name, phone and password are required")
		return
	}
	if ri.ParentCol != "" && b.ParentID == nil {
		fail(w, 400, "parent_id is required for role "+b.Role)
		return
	}
	if ri.ParentCol != "" {
		var exists bool
		if err := a.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM "+ri.ParentTable+" WHERE "+roles[parentRoleOf(b.Role)].IDCol+"=$1)", *b.ParentID).Scan(&exists); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if !exists {
			fail(w, 400, "parent "+strconv.Itoa(*b.ParentID)+" does not exist")
			return
		}
	}

	hash, err := auth.HashPassword(b.Password)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}

	var newID int
	idExpr := fmt.Sprintf("(SELECT COALESCE(MAX(%s),%d)+1 FROM %s)", ri.IDCol, baseID(b.Role), ri.Table)
	cols := ri.IDCol + ", name, phone, area"
	vals := idExpr + ", $1, $2, $3"
	args := []any{b.Name, b.Phone, b.Area}
	if ri.ParentCol != "" {
		cols += ", " + ri.ParentCol
		vals += ", $4"
		args = append(args, *b.ParentID)
	}
	err = a.DB.QueryRow(
		"INSERT INTO "+ri.Table+" ("+cols+") "+fmt.Sprintf("VALUES (%s) RETURNING %s", vals, ri.IDCol),
		args...,
	).Scan(&newID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(w, 409, "phone already in use")
			return
		}
		fail(w, 500, err.Error())
		return
	}
	if _, err := a.DB.Exec(`INSERT INTO credential (role, login_id, password_hash) VALUES ($1,$2,$3)`, b.Role, newID, hash); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"id": newID})
}

// baseID is the starting id for each role's auto-numbering.
func baseID(role string) int {
	switch role {
	case "regional_coordinator":
		return 101
	case "district_coordinator":
		return 201
	case "supervisor":
		return 301
	default:
		return 1001
	}
}

// parentRoleOf maps a role to its parent's role.
func parentRoleOf(role string) string {
	switch role {
	case "executive":
		return "supervisor"
	case "supervisor":
		return "district_coordinator"
	case "district_coordinator":
		return "regional_coordinator"
	default:
		return ""
	}
}

// updateMember PUT /admin/members/{role}/{id}  {name,phone,area,parent_id}
func (a *API) updateMember(w http.ResponseWriter, r *http.Request) {
	role := r.PathValue("role")
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		fail(w, 400, "bad id")
		return
	}
	ri, ok := roleMeta(role)
	if !ok {
		fail(w, 400, "unknown role")
		return
	}
	var b memberBody
	if err := decode(r, &b); err != nil {
		fail(w, 400, err.Error())
		return
	}

	sets := []string{"name=$1", "phone=$2", "area=$3"}
	args := []any{b.Name, b.Phone, b.Area}
	if ri.ParentCol != "" {
		if b.ParentID == nil {
			fail(w, 400, "parent_id required")
			return
		}
		sets = append(sets, ri.ParentCol+"=$"+strconv.Itoa(len(args)+1))
		args = append(args, *b.ParentID)
	}
	args = append(args, id)
	res, err := a.DB.Exec("UPDATE "+ri.Table+" SET "+strings.Join(sets, ", ")+" WHERE "+ri.IDCol+"=$"+strconv.Itoa(len(args)), args...)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "member not found")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// resetPassword PUT /admin/members/{role}/{id}/password {password}
func (a *API) resetPassword(w http.ResponseWriter, r *http.Request) {
	role := r.PathValue("role")
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		fail(w, 400, "bad id")
		return
	}
	if _, ok := roleMeta(role); !ok {
		fail(w, 400, "unknown role")
		return
	}
	var b memberBody
	if err := decode(r, &b); err != nil || b.Password == "" {
		fail(w, 400, "password required")
		return
	}
	hash, err := auth.HashPassword(b.Password)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	res, err := a.DB.Exec(`UPDATE credential SET password_hash=$3 WHERE role=$1 AND login_id=$2`, role, id, hash)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "member credential not found")
		return
	}
	// log the member out everywhere after a password change
	a.DB.Exec(`DELETE FROM session WHERE role=$1 AND login_id=$2`, role, strconv.Itoa(id))
	writeJSON(w, 200, map[string]any{"ok": true})
}

// deleteMember DELETE /admin/members/{role}/{id}
func (a *API) deleteMember(w http.ResponseWriter, r *http.Request) {
	role := r.PathValue("role")
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		fail(w, 400, "bad id")
		return
	}
	ri, ok := roleMeta(role)
	if !ok {
		fail(w, 400, "unknown role")
		return
	}

	tx, err := a.DB.Begin()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	if role == "executive" {
		if _, err := tx.Exec(`DELETE FROM daily_activity WHERE ex_id=$1`, id); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	for _, stmt := range []struct {
		q    string
		args []any
	}{
		{`DELETE FROM payout WHERE role=$1 AND login_id=$2`, []any{role, id}},
		{`DELETE FROM credential WHERE role=$1 AND login_id=$2`, []any{role, id}},
		{`DELETE FROM session WHERE role=$1 AND login_id=$2`, []any{role, strconv.Itoa(id)}},
		{"DELETE FROM " + ri.Table + " WHERE " + ri.IDCol + "=$1", []any{id}},
	} {
		if _, err := tx.Exec(stmt.q, stmt.args...); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
