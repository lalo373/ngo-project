package config

import (
	"bufio"
	"os"
	"strings"
)

// IST is Indian Standard Time (no DST, so a fixed offset is safe).
var IST = "Asia/Kolkata"

// Config holds runtime settings. Business rules live here so they are
// easy to change in one place.
type Config struct {
	Port  string
	DBURL string

	AdminUser string
	AdminPass string

	// Executive eligibility: logged in between 9:00 and 17:00 IST
	// AND target_count >= ExecMinTarget.
	ExecMinTarget  int     // 25
	ExecTierTarget int     // 45
	ExecBaseAmount float64 // 25 <= targets < 45 -> 500
	ExecTierAmount float64 // >= 45 -> 1000

	// Supervisor: paid when >= SVThreshold of their executives are eligible.
	SVThreshold      float64 // 0.70
	SupervisorAmount float64 // 500

	// District coordinator: paid when >= DCThreshold of their supervisors are eligible.
	DCThreshold float64 // 0.85
	DCAmount    float64 // 750

	// Payout computation runs daily at this hour (IST).
	ComputeHour int // 20 (8 PM)
}

func Load() *Config {
	loadDotEnv(".env")
	c := &Config{
		Port:             env("PORT", "5001"),
		DBURL:            env("DBURL", ""),
		AdminUser:        env("ADMIN_USER", "admin"),
		AdminPass:        env("ADMIN_PASS", "admin@123"),
		ExecMinTarget:    25,
		ExecTierTarget:   45,
		ExecBaseAmount:   500,
		ExecTierAmount:   1000,
		SVThreshold:      0.70,
		SupervisorAmount: 500,
		DCThreshold:      0.85,
		DCAmount:         750,
		ComputeHour:      20,
	}
	return c
}

// loadDotEnv reads simple KEY="VALUE" pairs from a file if it exists.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
}

// WorkStartMinutes returns the eligibility window start (9:00) in minutes since midnight.
func (c *Config) WorkStartMinutes() int { return 9 * 60 }

// WorkEndMinutes returns the eligibility window end (17:00) in minutes since midnight.
func (c *Config) WorkEndMinutes() int { return 17 * 60 }

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
