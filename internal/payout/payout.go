// Package payout implements the daily eligibility engine.
//
// Every day at 20:00 IST it computes, for that date, which executives,
// supervisors and district coordinators are eligible for a payout, and
// writes one row per member into the payout table. Members query these
// rows through the API; the weekly payout is simply the sum of the
// daily amounts within a Monday-Sunday week.
//
// The per-member work is fanned out across a bounded worker pool, which
// keeps the computation fast at scale (~10,000 members/day).
package payout

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"payroll/internal/auth"
	"payroll/internal/config"
)

type Engine struct {
	DB  *sql.DB
	Cfg *config.Config
}

func New(db *sql.DB, cfg *config.Config) *Engine {
	return &Engine{DB: db, Cfg: cfg}
}

// ---- rule inputs -------------------------------------------------------

// activity is one executive's raw data for a single day.
type activity struct {
	LoginTime   time.Time
	HasLogin    bool
	TargetCount int
}

type execInput struct {
	ID   int
	SvID sql.NullInt64
}

type memberRow struct {
	Role     string
	ID       int
	Eligible bool
	Amount   float64
	Metric   float64 // executives: target_count; supervisors/DCs: eligible percent
}

// ---- eligibility rules -------------------------------------------------

// execEligible applies the executive rule: logged in between 9:00 and
// 17:00 IST AND target_count >= 25.
func (e *Engine) execEligible(a activity) bool {
	if !a.HasLogin || a.TargetCount < e.Cfg.ExecMinTarget {
		return false
	}
	h, m, _ := a.LoginTime.In(auth.IST).Clock()
	start := e.Cfg.WorkStartMinutes()
	end := e.Cfg.WorkEndMinutes()
	now := h*60 + m
	return now >= start && now < end
}

func (e *Engine) execAmount(target int) float64 {
	if target >= e.Cfg.ExecTierTarget {
		return e.Cfg.ExecTierAmount
	}
	return e.Cfg.ExecBaseAmount
}

// ---- computation -------------------------------------------------------

// ComputeDate computes and upserts payouts for a given date (IST).
// Safe to run more than once: rows are upserted.
func (e *Engine) ComputeDate(ctx context.Context, date time.Time) (int, error) {
	day := date.In(auth.IST)
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, auth.IST)
	dayEnd := dayStart.Add(24 * time.Hour)

	// Load raw inputs.
	acts, err := e.loadActivities(ctx, dayStart, dayEnd)
	if err != nil {
		return 0, fmt.Errorf("load activities: %w", err)
	}
	execs, err := e.loadExecutives(ctx)
	if err != nil {
		return 0, fmt.Errorf("load executives: %w", err)
	}
	svs, err := e.loadSupervisors(ctx)
	if err != nil {
		return 0, fmt.Errorf("load supervisors: %w", err)
	}
	dcs, err := e.loadDistrictCoordinators(ctx)
	if err != nil {
		return 0, fmt.Errorf("load district coordinators: %w", err)
	}

	// 1) Executives: fan the per-member rule check out over a worker pool.
	rows := make([]memberRow, len(execs))
	in := make(chan int)
	var wg sync.WaitGroup
	workers := 32
	if len(execs) < workers {
		workers = len(execs)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range in {
				ex := execs[idx]
				a := acts[ex.ID]
				elig := e.execEligible(a)
				amount := 0.0
				if elig {
					amount = e.execAmount(a.TargetCount)
				}
				rows[idx] = memberRow{
					Role:     "executive",
					ID:       ex.ID,
					Eligible: elig,
					Amount:   amount,
					Metric:   float64(a.TargetCount),
				}
			}
		}()
	}
	for i := range execs {
		in <- i
	}
	close(in)
	wg.Wait()

	// 2) Supervisors: >= 70% of their executives eligible.
	eligBySv, totalBySv := map[int]int{}, map[int]int{}
	for i, ex := range execs {
		if !ex.SvID.Valid {
			continue
		}
		sv := int(ex.SvID.Int64)
		totalBySv[sv]++
		if rows[i].Eligible {
			eligBySv[sv]++
		}
	}
	for _, sv := range svs {
		total := totalBySv[sv.ID]
		elig := 0
		if total > 0 {
			elig = eligBySv[sv.ID]
		}
		pct := 0.0
		if total > 0 {
			pct = float64(elig) / float64(total) * 100
		}
		isElig := total > 0 && float64(elig)/float64(total) >= e.Cfg.SVThreshold
		amount := 0.0
		if isElig {
			amount = e.Cfg.SupervisorAmount
		}
		rows = append(rows, memberRow{Role: "supervisor", ID: sv.ID, Eligible: isElig, Amount: amount, Metric: pct})
	}

	// 3) District coordinators: >= 85% of their supervisors eligible.
	eligByDc, totalByDc := map[int]int{}, map[int]int{}
	for _, sv := range svs {
		if !sv.DcID.Valid {
			continue
		}
		dc := int(sv.DcID.Int64)
		totalByDc[dc]++
		for _, r := range rows {
			if r.Role == "supervisor" && r.ID == sv.ID && r.Eligible {
				eligByDc[dc]++
				break
			}
		}
	}
	for _, dc := range dcs {
		total := totalByDc[dc.ID]
		elig := eligByDc[dc.ID]
		pct := 0.0
		if total > 0 {
			pct = float64(elig) / float64(total) * 100
		}
		isElig := total > 0 && float64(elig)/float64(total) >= e.Cfg.DCThreshold
		amount := 0.0
		if isElig {
			amount = e.Cfg.DCAmount
		}
		rows = append(rows, memberRow{Role: "district_coordinator", ID: dc.ID, Eligible: isElig, Amount: amount, Metric: pct})
	}

	// 4) Persist (upsert) in one transaction, batched into multi-row
	// statements to keep round trips low.
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	const chunk = 300
	for start := 0; start < len(rows); start += chunk {
		end := start + chunk
		if end > len(rows) {
			end = len(rows)
		}
		part := rows[start:end]
		var sb strings.Builder
		sb.WriteString(`INSERT INTO payout (payout_date, role, login_id, eligible, amount, metric, computed_at) VALUES `)
		args := make([]any, 0, len(part)*6)
		for i, r := range part {
			if i > 0 {
				sb.WriteString(",")
			}
			base := i * 6
			fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,now())", base+1, base+2, base+3, base+4, base+5, base+6)
			args = append(args, dayStart, r.Role, r.ID, r.Eligible, r.Amount, r.Metric)
		}
		sb.WriteString(` ON CONFLICT (payout_date, role, login_id)
			DO UPDATE SET eligible=EXCLUDED.eligible, amount=EXCLUDED.amount, metric=EXCLUDED.metric, computed_at=now()`)
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// ---- loaders -----------------------------------------------------------

func (e *Engine) loadActivities(ctx context.Context, start, end time.Time) (map[int]activity, error) {
	rs, err := e.DB.QueryContext(ctx, `SELECT ex_id, login_time, target_count FROM daily_activity WHERE activity_date = $1`, start)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	m := map[int]activity{}
	for rs.Next() {
		var id int
		var a activity
		var lt sql.NullTime
		if err := rs.Scan(&id, &lt, &a.TargetCount); err != nil {
			return nil, err
		}
		a.HasLogin = lt.Valid
		a.LoginTime = lt.Time
		m[id] = a
	}
	return m, rs.Err()
}

type supervisorRow struct {
	ID   int
	DcID sql.NullInt64
}

func (e *Engine) loadExecutives(ctx context.Context) ([]execInput, error) {
	rs, err := e.DB.QueryContext(ctx, `SELECT ex_id, sv_id FROM executive ORDER BY ex_id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []execInput
	for rs.Next() {
		var ex execInput
		if err := rs.Scan(&ex.ID, &ex.SvID); err != nil {
			return nil, err
		}
		out = append(out, ex)
	}
	return out, rs.Err()
}

func (e *Engine) loadSupervisors(ctx context.Context) ([]supervisorRow, error) {
	rs, err := e.DB.QueryContext(ctx, `SELECT sv_id, dc_id FROM supervisor ORDER BY sv_id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []supervisorRow
	for rs.Next() {
		var sv supervisorRow
		if err := rs.Scan(&sv.ID, &sv.DcID); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rs.Err()
}

func (e *Engine) loadDistrictCoordinators(ctx context.Context) ([]struct{ ID int }, error) {
	rs, err := e.DB.QueryContext(ctx, `SELECT dc_id FROM district_coordinator ORDER BY dc_id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []struct{ ID int }
	for rs.Next() {
		var dc struct{ ID int }
		if err := rs.Scan(&dc.ID); err != nil {
			return nil, err
		}
		out = append(out, dc)
	}
	return out, rs.Err()
}

// ---- scheduler ---------------------------------------------------------

// RunScheduler computes payouts every day at 20:00 IST until ctx is done.
func (e *Engine) RunScheduler(ctx context.Context) {
	for {
		next := e.nextRunTime(time.Now().In(auth.IST))
		log.Printf("payout: next computation at %s", next.Format(time.RFC3339))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		n, err := e.ComputeDate(ctx, next)
		if err != nil {
			log.Printf("payout: compute failed: %v", err)
			continue
		}
		log.Printf("payout: computed %d rows for %s", n, next.Format("2006-01-02"))
	}
}

// nextRunTime returns the next 20:00 IST instant at or after t.
func (e *Engine) nextRunTime(t time.Time) time.Time {
	run := time.Date(t.Year(), t.Month(), t.Day(), e.Cfg.ComputeHour, 0, 0, 0, auth.IST)
	if !t.Before(run) {
		run = run.AddDate(0, 0, 1)
	}
	return run
}
