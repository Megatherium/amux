// Copyright (C) 2026 megatherium
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package dolt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/andyrewlee/amux/internal/tickets"
)

type Mode string

const ServerMode Mode = "server"

type ServerStore struct {
	db        *sql.DB
	closed    bool
	beadsDir  string
	metadata  *Metadata
	autostart bool
	mode      Mode
}

var _ tickets.TicketStore = (*ServerStore)(nil)

func (s *ServerStore) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *ServerStore) DB() *sql.DB {
	return s.db
}

func (s *ServerStore) CanRetryConnection() bool {
	return s.mode == ServerMode
}

func (s *ServerStore) AutostartEnabled() bool {
	return s.autostart
}

func (s *ServerStore) EnsureRunningAgentsTable(ctx context.Context) error {
	return nil
}

type ErrServerNotRunningStore struct {
	Message string
}

func (e *ErrServerNotRunningStore) Error() string {
	return e.Message
}

func IsErrServerNotRunningStore(err error) bool {
	var e *ErrServerNotRunningStore
	return errors.As(err, &e)
}

func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "cannot connect to Dolt server") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "No connection could be made") ||
		strings.Contains(errStr, "dial tcp")
}

func handleServerMode(ctx context.Context, beadsDir string, metadata *Metadata, autostart bool) (*ServerStore, error) {
	store, err := newStore(ctx, beadsDir, metadata, autostart)
	if err != nil {
		if !IsConnectionError(err) {
			return nil, err
		}

		if !autostart {
			return nil, &ErrServerNotRunningStore{
				Message: "Dolt server is not running. Start dolt server? [y/N]",
			}
		}

		if startErr := StartServer(ctx, beadsDir, metadata); startErr != nil {
			return nil, fmt.Errorf("failed to auto-start dolt server: %w", startErr)
		}

		return newServerStoreWithMode(ctx, beadsDir, metadata, autostart)
	}

	return store, nil
}

func newServerStoreWithMode(ctx context.Context, beadsDir string, metadata *Metadata, autostart bool) (*ServerStore, error) {
	store, err := newStore(ctx, beadsDir, metadata, autostart)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func NewStore(ctx context.Context, beadsDir string, autostart bool) (*ServerStore, error) {
	if beadsDir == "" {
		beadsDir = ".beads"
	}

	metadata, err := LoadMetadata(beadsDir)
	if err != nil {
		return nil, err
	}

	return handleServerMode(ctx, beadsDir, metadata, autostart)
}

func (s *ServerStore) TryStartServer(ctx context.Context) (*ServerStore, error) {
	if s.mode != ServerMode {
		return nil, errors.New("server restart not supported for this store mode")
	}

	if startErr := StartServer(ctx, s.beadsDir, s.metadata); startErr != nil {
		return nil, fmt.Errorf("failed to start dolt server: %w", startErr)
	}

	return newServerStoreWithMode(ctx, s.beadsDir, s.metadata, s.autostart)
}

func (s *ServerStore) ListTickets(ctx context.Context, filter tickets.TicketFilter) ([]tickets.Ticket, error) {
	if s.closed {
		return nil, errors.New("store is closed")
	}

	if s.db == nil {
		return nil, errors.New("store has no database connection")
	}

	query, args := buildListTicketsQuery(filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		if s.mode == ServerMode && IsConnectionError(err) {
			if s.autostart {
				return nil, &ErrServerNotRunningStore{
					Message: "Dolt server connection failed. Would you like to restart it?",
				}
			}
			return nil, &ErrServerNotRunningStore{
				Message: "Dolt server connection failed. Please check that it's running.",
			}
		}
		return nil, fmt.Errorf("failed to query tickets: %w", err)
	}
	defer rows.Close()

	return scanTickets(rows)
}

func (s *ServerStore) LatestUpdate(ctx context.Context) (time.Time, error) {
	if s.closed {
		return time.Time{}, errors.New("store is closed")
	}

	if s.db == nil {
		return time.Time{}, errors.New("store has no database connection")
	}

	var latest sql.NullTime
	query := "SELECT MAX(updated_at) FROM ready_issues"
	err := s.db.QueryRowContext(ctx, query).Scan(&latest)
	if err != nil {
		if s.mode == ServerMode && IsConnectionError(err) {
			if s.autostart {
				return time.Time{}, &ErrServerNotRunningStore{
					Message: "Dolt server connection failed. Would you like to restart it?",
				}
			}
			return time.Time{}, &ErrServerNotRunningStore{
				Message: "Dolt server connection failed. Please check that it's running.",
			}
		}
		return time.Time{}, fmt.Errorf("failed to query latest update: %w", err)
	}

	if !latest.Valid {
		return time.Time{}, nil
	}

	return latest.Time, nil
}

func buildListTicketsQuery(filter tickets.TicketFilter) (query string, args []any) {
	var sb strings.Builder
	sb.WriteString(`SELECT id, title, description, status, priority, issue_type, assignee, created_at, updated_at FROM ready_issues WHERE 1=1`)

	if filter.Status != "" {
		sb.WriteString(" AND status = ?")
		args = append(args, filter.Status)
	}

	if filter.IssueType != "" {
		sb.WriteString(" AND issue_type = ?")
		args = append(args, filter.IssueType)
	}

	if filter.Search != "" {
		sb.WriteString(" AND title LIKE ?")
		args = append(args, "%"+filter.Search+"%")
	}

	sb.WriteString(" ORDER BY priority ASC, updated_at DESC")

	if filter.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}

	return sb.String(), args
}

func scanTickets(rows *sql.Rows) ([]tickets.Ticket, error) {
	var result []tickets.Ticket

	for rows.Next() {
		var t tickets.Ticket
		var assignee sql.NullString

		err := rows.Scan(
			&t.ID,
			&t.Title,
			&t.Description,
			&t.Status,
			&t.Priority,
			&t.IssueType,
			&assignee,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket row: %w", err)
		}

		if assignee.Valid {
			t.Assignee = assignee.String
		}

		result = append(result, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ticket rows: %w", err)
	}

	return result, nil
}

func newStore(ctx context.Context, beadsDir string, metadata *Metadata, autostart bool) (*ServerStore, error) {
	resolvedPort, err := metadata.ResolveServerPort(beadsDir)
	if err != nil && autostart {
		resolvedPort = 0
	}

	if resolvedPort == 0 && !autostart {
		return nil, &ErrServerNotRunning{
			Message: "Dolt server is not running and port could not be resolved. Start dolt server? [y/N]",
		}
	}

	if resolvedPort > 0 {
		metadata.ServerPort = resolvedPort
	}

	dsn := buildServerDSN(metadata, resolvedPort)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create MySQL connection pool: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf(
			"cannot connect to Dolt server at %s:%d: %w; "+
				"check that the server is running and accessible",
			metadata.ServerHost, resolvedPort, err,
		)
	}

	if err := verifySchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &ServerStore{
		db:        db,
		beadsDir:  beadsDir,
		metadata:  metadata,
		autostart: autostart,
		mode:      ServerMode,
	}

	if err := store.EnsureRunningAgentsTable(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func buildServerDSN(metadata *Metadata, port int) string {
	if port == 0 {
		panic("buildServerDSN called with port=0: port must be resolved before calling buildServerDSN")
	}

	cfg := mysql.NewConfig()

	cfg.Net = "tcp"
	cfg.User = metadata.ServerUser
	if cfg.User == "" {
		cfg.User = "root"
	}

	host := metadata.ServerHost
	if host == "" {
		host = "127.0.0.1"
	}
	cfg.Addr = fmt.Sprintf("%s:%d", host, port)

	cfg.Passwd = os.Getenv("BEADS_DOLT_PASSWORD")
	cfg.DBName = metadata.DoltDatabase
	cfg.ParseTime = true
	cfg.Loc = time.UTC

	return cfg.FormatDSN()
}

type ErrServerNotRunning struct {
	Message string
}

func (e *ErrServerNotRunning) Error() string {
	return e.Message
}

func IsErrServerNotRunning(err error) bool {
	var e *ErrServerNotRunning
	return errors.As(err, &e)
}

func StartServer(ctx context.Context, beadsDir string, metadata *Metadata) error {
	projectDir := filepath.Dir(beadsDir)
	timeout := metadata.ServerReadyTimeout()

	startCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(startCtx, "bd", "dolt", "start")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dolt server: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		if startCtx.Err() == context.DeadlineExceeded {
		} else {
			return fmt.Errorf("dolt server start command failed: %w", err)
		}
	}

	return waitForServerReady(ctx, beadsDir, timeout)
}

func waitForServerReady(ctx context.Context, beadsDir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		port, err := readServerPort(beadsDir)
		if err == nil && port > 0 {
			if err := testServerConnection(ctx, beadsDir, port); err == nil {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("dolt server did not become ready within %v", timeout)
}

func readServerPort(beadsDir string) (int, error) {
	portFilePath := filepath.Join(beadsDir, doltServerPortFile)

	data, err := os.ReadFile(portFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read port file: %w", err)
	}

	portStr := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port in file: %s", portStr)
	}

	return port, nil
}

func testServerConnection(ctx context.Context, beadsDir string, port int) error {
	metadata, err := LoadMetadata(beadsDir)
	if err != nil {
		return err
	}

	dsn := buildServerDSN(metadata, port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return db.PingContext(pingCtx)
}

func TryStartServer(ctx context.Context, beadsDir string, metadata *Metadata) (*ServerStore, error) {
	if err := StartServer(ctx, beadsDir, metadata); err != nil {
		return nil, fmt.Errorf("failed to start dolt server: %w", err)
	}

	return newStore(ctx, beadsDir, metadata, true)
}
