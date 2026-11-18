package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Connect opens a pgx-backed *sql.DB that prefers IPv4 addresses
// (IPv6 is unavailable in some hosting sandboxes) and recycles
// connections so long-lived proxies do not strand stale sockets.
func Connect(dburl string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dburl)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// Prefer IPv4 when the host resolves to both families.
	origDial := cfg.DialFunc
	cfg.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if conn, err := dialIPv4First(ctx, network, addr); err == nil {
			return conn, nil
		}
		return origDial(ctx, network, addr)
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("db.ping: %w", err)
	}
	return db, nil
}

// dialIPv4First resolves addr and dials the first reachable IPv4 address,
// falling back to any address (IPv6) if there is no A record.
func dialIPv4First(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); ip != nil {
		// literal IP: nothing to reorder
		return (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, addr)
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	v4 := make([]netip.Addr, 0, len(addrs))
	other := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		if a.Is4() || a.Is4In6() {
			v4 = append(v4, a)
		} else {
			other = append(other, a)
		}
	}
	ordered := append(v4, other...)
	if len(ordered) == 0 {
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, a := range ordered {
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(a.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
