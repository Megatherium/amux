package dolt

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestScanTickets(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	now := time.Now()

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-001", "Test", "Description", "open", 1, "task", nil, now, now, nil).
			AddRow("bb-002", "Assigned", "Work", "in_progress", 2, "feature", "dev@example.com", now, now, nil))

	rows, err := db.Query(`SELECT`)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	ticketList, err := scanTickets(rows)
	if err != nil {
		t.Fatalf("failed to scan tickets: %v", err)
	}

	if len(ticketList) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(ticketList))
	}

	if ticketList[0].Assignee != "" {
		t.Errorf("expected empty assignee for first ticket, got %q", ticketList[0].Assignee)
	}

	if ticketList[1].Assignee != "dev@example.com" {
		t.Errorf("expected assignee 'dev@example.com', got %q", ticketList[1].Assignee)
	}

	if ticketList[0].ID != "bb-001" || ticketList[1].ID != "bb-002" {
		t.Error("ticket IDs don't match expected")
	}
}

func TestScanTickets_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}))

	rows, err := db.Query(`SELECT`)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	ticketList, err := scanTickets(rows)
	if err != nil {
		t.Fatalf("failed to scan tickets: %v", err)
	}

	if len(ticketList) != 0 {
		t.Errorf("expected 0 tickets, got %d", len(ticketList))
	}
}

func TestErrServerNotRunningStore(t *testing.T) {
	err := &ErrServerNotRunningStore{Message: "test message"}

	if err.Error() != "test message" {
		t.Errorf("Expected 'test message', got %q", err.Error())
	}
}

func TestIsErrServerNotRunningStore(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ErrServerNotRunningStore",
			err:      &ErrServerNotRunningStore{Message: "test"},
			expected: true,
		},
		{
			name:     "wrapped ErrServerNotRunningStore",
			err:      errors.New("wrapped: %w"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsErrServerNotRunningStore(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "cannot connect message",
			errMsg:   "cannot connect to Dolt server",
			expected: true,
		},
		{
			name:     "connection refused",
			errMsg:   "connection refused",
			expected: true,
		},
		{
			name:     "no connection could be made",
			errMsg:   "No connection could be made",
			expected: true,
		},
		{
			name:     "dial tcp",
			errMsg:   "dial tcp 127.0.0.1:3306",
			expected: true,
		},
		{
			name:     "generic error",
			errMsg:   "something went wrong",
			expected: false,
		},
		{
			name:     "empty error",
			errMsg:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = errors.New(tt.errMsg)
			}
			result := IsConnectionError(err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsConnectionError_NilError(t *testing.T) {
	result := IsConnectionError(nil)
	if result != false {
		t.Errorf("Expected false for nil error, got %v", result)
	}
}

func TestStore_Close_Idempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}

	store := &ServerStore{
		db:     db,
		mode:   ServerMode,
		closed: false,
	}

	mock.ExpectClose()
	if err := store.Close(); err != nil {
		t.Errorf("first Close() failed: %v", err)
	}

	if !store.closed {
		t.Error("store.closed should be true after Close()")
	}

	if err := store.Close(); err != nil {
		t.Errorf("second Close() should be idempotent but failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}

	db.Close()
}

func TestStore_Close_ReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}

	store := &ServerStore{
		db:     db,
		mode:   ServerMode,
		closed: false,
	}

	mock.ExpectClose().WillReturnError(errors.New("connection reset"))

	err = store.Close()
	if err == nil {
		t.Fatal("expected error from Close()")
	}

	if err.Error() != "connection reset" {
		t.Errorf("expected 'connection reset' error, got: %v", err)
	}

	if !store.closed {
		t.Error("store.closed should be true even when Close() returns error")
	}

	db.Close()
}

func TestStore_CanRetryConnection(t *testing.T) {
	store := &ServerStore{mode: ServerMode}
	if !store.CanRetryConnection() {
		t.Error("Expected CanRetryConnection to return true for ServerMode")
	}
}

func TestStore_AutostartEnabled(t *testing.T) {
	store := &ServerStore{
		autostart: true,
	}
	if !store.AutostartEnabled() {
		t.Error("Expected AutostartEnabled to return true")
	}

	store = &ServerStore{
		autostart: false,
	}
	if store.AutostartEnabled() {
		t.Error("Expected AutostartEnabled to return false")
	}
}

func TestStore_AutostartEnabled_DefaultsToFalse(t *testing.T) {
	store := &ServerStore{}
	if store.AutostartEnabled() {
		t.Error("Expected AutostartEnabled to return false for zero value Store")
	}
}

func TestStore_DB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	store := &ServerStore{
		db: db,
	}

	if store.DB() != db {
		t.Error("Expected DB() to return the underlying db")
	}
}

func TestStore_DB_NilDB(t *testing.T) {
	store := &ServerStore{
		db: nil,
	}

	if store.DB() != nil {
		t.Error("Expected DB() to return nil for nil db")
	}
}
