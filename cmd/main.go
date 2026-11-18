package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"payroll/internal/api"
	"payroll/internal/auth"
	"payroll/internal/config"
	"payroll/internal/db"
	"payroll/internal/payout"
	"payroll/internal/schema"
	"payroll/internal/seed"
)

func main() {
	seedFile := flag.String("seed", "", "path to an activities CSV; when set, seed the database and exit")
	flag.Parse()

	cfg := config.Load()
	if cfg.DBURL == "" {
		log.Fatal("DBURL not set (add it to .env)")
	}

	database, err := db.Connect(cfg.DBURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	if err := schema.Apply(database); err != nil {
		log.Fatalf("schema: %v", err)
	}

	if err := auth.EnsureAdmin(database, cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("admin: %v", err)
	}

	eng := payout.New(database, cfg)

	// one-shot seeding mode: go run ./cmd -seed data/dummy_activities.csv
	if *seedFile != "" {
		if err := seed.Run(database, *seedFile, eng); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Println("seed complete")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// daily 20:00 IST eligibility computation
	go eng.RunScheduler(ctx)

	// opportunistic cleanup of expired (24h) sessions every hour
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				auth.PurgeExpiredSessions(database)
			}
		}
	}()

	addr := ":" + cfg.Port
	if cfg.BindAddr != "" {
		addr = cfg.BindAddr + ":" + cfg.Port
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(database, cfg, eng),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("payroll listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
