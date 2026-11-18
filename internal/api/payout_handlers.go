package api

import (
	"net/http"
	"time"

	"payroll/internal/auth"
)

// dailyPayout returns today's computed payout for the logged-in member.
// GET /dailyPayout
func (a *API) dailyPayout(w http.ResponseWriter, r *http.Request) {
	m := currentMember(r)
	today := todayIST()

	var eligible bool
	var amount, metric float64
	err := a.DB.QueryRow(
		`SELECT eligible, amount, COALESCE(metric,0) FROM payout WHERE role=$1 AND login_id=$2 AND payout_date=$3`,
		m.Role, m.ID, today,
	).Scan(&eligible, &amount, &metric)
	if err != nil {
		// not computed yet (job runs at 20:00 IST) — report as not eligible
		eligible, amount, metric = false, 0, 0
	}

	// weekly total so far (current Mon-Sun week)
	var weekly float64
	a.DB.QueryRow(
		`SELECT COALESCE(SUM(amount),0) FROM payout WHERE role=$1 AND login_id=$2 AND eligible AND payout_date >= $3 AND payout_date <= $4`,
		m.Role, m.ID, weekStart(today), weekEnd(today),
	).Scan(&weekly)

	writeJSON(w, 200, map[string]any{
		"date":         today.Format("2006-01-02"),
		"eligible":     eligible,
		"amount":       amount,
		"metric":       metric,
		"weekly_total": weekly,
	})
}

// weeklyPayouts returns Monday-Sunday weeks in descending order, current
// week on top. GET /weeklyPayouts
func (a *API) weeklyPayouts(w http.ResponseWriter, r *http.Request) {
	m := currentMember(r)
	today := todayIST()
	since := weekStart(today).AddDate(0, 0, -7*12) // last 12 weeks

	rows, err := a.DB.Query(
		`SELECT payout_date, eligible, amount, COALESCE(metric,0) FROM payout WHERE role=$1 AND login_id=$2 AND payout_date >= $3 ORDER BY payout_date DESC`,
		m.Role, m.ID, since,
	)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type day struct {
		Date     string  `json:"date"`
		Eligible bool    `json:"eligible"`
		Amount   float64 `json:"amount"`
		Metric   float64 `json:"metric"` // executives: target_count; supervisors/DCs: eligible %
	}
	type week struct {
		WeekStart string  `json:"week_start"`
		WeekEnd   string  `json:"week_end"`
		Total     float64 `json:"total"`
		Days      []day   `json:"days"`
	}
	var current *week
	var weeks []*week
	for rows.Next() {
		var d time.Time
		var eligible bool
		var amount, metric float64
		if err := rows.Scan(&d, &eligible, &amount, &metric); err != nil {
			fail(w, 500, err.Error())
			return
		}
		ws := weekStart(d)
		if current == nil || current.WeekStart != ws.Format("2006-01-02") {
			current = &week{
				WeekStart: ws.Format("2006-01-02"),
				WeekEnd:   weekEnd(d).Format("2006-01-02"),
				Days:      []day{},
			}
			weeks = append(weeks, current)
		}
		current.Total += amount
		current.Days = append(current.Days, day{Date: d.Format("2006-01-02"), Eligible: eligible, Amount: amount, Metric: metric})
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if weeks == nil {
		weeks = []*week{}
	}
	writeJSON(w, 200, map[string]any{"weeks": weeks})
}

// calendar returns payouts for one month (query: month=YYYY-MM).
// GET /calendar
func (a *API) calendar(w http.ResponseWriter, r *http.Request) {
	m := currentMember(r)
	month := queryStrings(r, "month", todayIST().Format("2006-01"))
	start, err := time.Parse("2006-01", month)
	if err != nil {
		fail(w, 400, "month must be YYYY-MM")
		return
	}
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, auth.IST)
	end := start.AddDate(0, 1, 0)

	rows, err := a.DB.Query(
		`SELECT payout_date, amount FROM payout WHERE role=$1 AND login_id=$2 AND payout_date >= $3 AND payout_date < $4 ORDER BY payout_date`,
		m.Role, m.ID, start, end,
	)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type calDay struct {
		Date   string  `json:"date"`
		Amount float64 `json:"amount"`
	}
	days := []calDay{}
	for rows.Next() {
		var d time.Time
		var amount float64
		if err := rows.Scan(&d, &amount); err != nil {
			fail(w, 500, err.Error())
			return
		}
		days = append(days, calDay{Date: d.Format("2006-01-02"), Amount: amount})
	}
	if err := rows.Err(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"month": month, "days": days})
}
