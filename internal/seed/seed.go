// Package seed loads member rows and a dummy activities CSV into the
// database, then runs the payout engine for every date found in the CSV.
//
// Usage: go run ./cmd -seed data/dummy_activities.csv
package seed

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"payroll/internal/auth"
	"payroll/internal/payout"
)

// Default passwords given by "admin" for the dummy dataset.
const (
	ExecPassword = "exec@123"
	SVPassword   = "super@123"
	DCPassword   = "dc@123"
	RCPassword   = "rc@123"
)

// Run seeds members + activities from csvPath, then computes payouts for
// every date in the CSV. Safe to re-run (idempotent upserts).
func Run(db DB, csvPath string, eng *payout.Engine) error {
	if err := seedMembers(db); err != nil {
		return fmt.Errorf("members: %w", err)
	}
	dates, err := loadActivities(db, csvPath)
	if err != nil {
		return fmt.Errorf("activities: %w", err)
	}

	// compute payouts for every date concurrently
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	errs := make(chan error, len(dates))
	for _, d := range dates {
		wg.Add(1)
		go func(d time.Time) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			n, err := eng.ComputeDate(context.Background(), d)
			if err != nil {
				errs <- fmt.Errorf("compute %s: %w", d.Format("2006-01-02"), err)
				return
			}
			log.Printf("seed: computed %d payout rows for %s", n, d.Format("2006-01-02"))
		}(d)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

// DB is the minimal database surface the seeder needs.
type DB = *sql.DB

// ---- members -----------------------------------------------------------

// seedMembers inserts 1 regional coordinator, 1 district coordinator,
// 5 supervisors and 100 executives (20 per supervisor) with hashed
// default passwords. Batched into one statement per table + credentials.
func seedMembers(db DB) error {
	rcPw, err := auth.HashPassword(RCPassword)
	if err != nil {
		return err
	}
	dcPw, err := auth.HashPassword(DCPassword)
	if err != nil {
		return err
	}
	svPw, err := auth.HashPassword(SVPassword)
	if err != nil {
		return err
	}
	exPw, err := auth.HashPassword(ExecPassword)
	if err != nil {
		return err
	}

	areas := []string{"Zone A", "Zone B", "Zone C", "Zone D", "Zone E"}

	type member struct {
		role     string
		id       int
		name     string
		phone    string
		area     string
		parentID *int
		password string
	}
	var members []member
	rcID, dcID := 101, 201
	members = append(members,
		member{"regional_coordinator", rcID, "Ramesh Kumar", "9000000101", "North Region", nil, rcPw},
		member{"district_coordinator", dcID, "Anita Sharma", "9000000201", "North District", &rcID, dcPw},
	)
	for i := 0; i < 5; i++ {
		id := 301 + i
		members = append(members, member{"supervisor", id,
			fmt.Sprintf("Supervisor %d", i+1), fmt.Sprintf("90000003%02d", i+1), areas[i], &dcID, svPw})
	}
	for i := 0; i < 100; i++ {
		id := 1001 + i
		sv := 301 + i/20
		members = append(members, member{"executive", id,
			fmt.Sprintf("Executive %03d", i+1), fmt.Sprintf("900000%04d", 1000+i+1), fmt.Sprintf("Ward %d", i%10+1), &sv, exPw})
	}

	tables := map[string]struct{ table, idCol, cols string }{
		"regional_coordinator": {"regional_coordinator", "rc_id", "(rc_id, name, phone, area)"},
		"district_coordinator": {"district_coordinator", "dc_id", "(dc_id, name, phone, rc_id, area)"},
		"supervisor":           {"supervisor", "sv_id", "(sv_id, name, phone, dc_id, area)"},
		"executive":            {"executive", "ex_id", "(ex_id, name, phone, sv_id, area)"},
	}

	// group by role so each role gets one INSERT with many VALUES rows
	byRole := map[string][]member{}
	for _, m := range members {
		byRole[m.role] = append(byRole[m.role], m)
	}
	for role, list := range byRole {
		t := tables[role]
		var sb strings.Builder
		fmt.Fprintf(&sb, "INSERT INTO %s %s VALUES ", t.table, t.cols)
		args := make([]any, 0, len(list)*5)
		p := 1 // running postgres parameter number
		for i, m := range list {
			if i > 0 {
				sb.WriteString(",")
			}
			if m.parentID == nil && role == "regional_coordinator" {
				// (id, name, phone, area)
				fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d)", p, p+1, p+2, p+3)
				args = append(args, m.id, m.name, m.phone, m.area)
				p += 4
			} else {
				// (id, name, phone, parent_id, area)
				fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d)", p, p+1, p+2, p+3, p+4)
				args = append(args, m.id, m.name, m.phone, m.parentID, m.area)
				p += 5
			}
		}
		sb.WriteString(" ON CONFLICT (" + t.idCol + ") DO NOTHING")
		if _, err := db.Exec(sb.String(), args...); err != nil {
			log.Printf("seed: failing SQL: %s | args=%d", sb.String(), len(args))
			return fmt.Errorf("%s: %w", role, err)
		}
	}

	// credentials: one INSERT ... ON CONFLICT DO NOTHING per distinct password role
	var cred strings.Builder
	cred.WriteString("INSERT INTO credential (role, login_id, password_hash) VALUES ")
	args := make([]any, 0, len(members)*3)
	for i, m := range members {
		if i > 0 {
			cred.WriteString(",")
		}
		base := i * 3
		fmt.Fprintf(&cred, "($%d,$%d,$%d)", base+1, base+2, base+3)
		args = append(args, m.role, m.id, m.password)
	}
	cred.WriteString(" ON CONFLICT (role, login_id) DO NOTHING")
	if _, err := db.Exec(cred.String(), args...); err != nil {
		return fmt.Errorf("credentials: %w", err)
	}
	return nil
}

// ---- activities CSV ----------------------------------------------------

// loadActivities reads `date,ex_id,login_time,target_count` rows into
// daily_activity and returns the distinct dates (in CSV order).
func loadActivities(db DB, csvPath string) ([]time.Time, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 4
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if strings.Join(header, ",") != "date,ex_id,login_time,target_count" {
		return nil, fmt.Errorf("unexpected CSV header %q, want date,ex_id,login_time,target_count", strings.Join(header, ","))
	}

	seen := map[string]bool{}
	var dates []time.Time
	const batch = 200
	var pending []struct {
		day    time.Time
		id     int
		login  any
		target int
	}
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		var sb strings.Builder
		sb.WriteString(`INSERT INTO daily_activity (activity_date, ex_id, login_time, target_count) VALUES `)
		args := make([]any, 0, len(pending)*4)
		for i, p := range pending {
			if i > 0 {
				sb.WriteString(",")
			}
			base := i * 4
			fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4)
			args = append(args, p.day, p.id, p.login, p.target)
		}
		sb.WriteString(` ON CONFLICT (activity_date, ex_id) DO UPDATE SET login_time=EXCLUDED.login_time, target_count=EXCLUDED.target_count`)
		if _, err := db.Exec(sb.String(), args...); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		dateStr, idStr, loginStr, targetStr := rec[0], rec[1], rec[2], rec[3]
		day, err := time.ParseInLocation("2006-01-02", dateStr, auth.IST)
		if err != nil {
			return nil, fmt.Errorf("row date %q: %w", dateStr, err)
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, fmt.Errorf("row ex_id %q: %w", idStr, err)
		}
		target, err := strconv.Atoi(targetStr)
		if err != nil {
			return nil, fmt.Errorf("row target_count %q: %w", targetStr, err)
		}
		var login any
		if loginStr != "" {
			t, err := time.ParseInLocation("2006-01-02 15:04", dateStr+" "+loginStr, auth.IST)
			if err != nil {
				return nil, fmt.Errorf("row login_time %q: %w", loginStr, err)
			}
			login = t
		}
		pending = append(pending, struct {
			day    time.Time
			id     int
			login  any
			target int
		}{day, id, login, target})
		if !seen[dateStr] {
			seen[dateStr] = true
			dates = append(dates, day)
		}
		if len(pending) >= batch {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return dates, nil
}
