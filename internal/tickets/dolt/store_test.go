package dolt

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/andyrewlee/amux/internal/tickets"
)

func newMockStore(t *testing.T) (*ServerStore, sqlmock.Sqlmock, *sql.DB) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}

	store := &ServerStore{
		db:     db,
		mode:   ServerMode,
		closed: false,
	}

	return store, mock, db
}

func TestStore_ImplementsTicketStore(t *testing.T) {
	var _ tickets.TicketStore = (*ServerStore)(nil)
}

func TestStore_ListTickets_NoFilter(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 ORDER BY priority ASC, updated_at DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-001", "Test Ticket", "Description", "open", 1, "task", nil, time.Now(), time.Now(), nil).
			AddRow("bb-002", "Another Ticket", "Another desc", "open", 2, "feature", "user@example.com", time.Now(), time.Now(), nil))

	filter := tickets.TicketFilter{}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(ticketList))
	}

	if ticketList[0].ID != "bb-001" {
		t.Errorf("expected first ticket ID 'bb-001', got %q", ticketList[0].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_WithStatusFilter(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND status = \? ORDER BY priority ASC, updated_at DESC`).
		WithArgs("closed").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-003", "Closed Ticket", "Done", "closed", 1, "task", nil, time.Now(), time.Now(), nil))

	filter := tickets.TicketFilter{Status: "closed"}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(ticketList))
	}

	if ticketList[0].Status != "closed" {
		t.Errorf("expected status 'closed', got %q", ticketList[0].Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_WithIssueTypeFilter(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND issue_type = \? ORDER BY priority ASC, updated_at DESC`).
		WithArgs("feature").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-004", "Feature Ticket", "New feature", "open", 1, "feature", nil, time.Now(), time.Now(), nil))

	filter := tickets.TicketFilter{IssueType: "feature"}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(ticketList))
	}

	if ticketList[0].IssueType != "feature" {
		t.Errorf("expected issue_type 'feature', got %q", ticketList[0].IssueType)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_WithSearchFilter(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND title LIKE \? ORDER BY priority ASC, updated_at DESC`).
		WithArgs("%test%").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-005", "Test Ticket", "A test", "open", 1, "task", nil, time.Now(), time.Now(), nil).
			AddRow("bb-006", "Testing Again", "Another test", "open", 2, "task", nil, time.Now(), time.Now(), nil))

	filter := tickets.TicketFilter{Search: "test"}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(ticketList))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_WithLimit(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 ORDER BY priority ASC, updated_at DESC LIMIT \?`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-007", "Ticket 1", "Desc 1", "open", 1, "task", nil, time.Now(), time.Now(), nil).
			AddRow("bb-008", "Ticket 2", "Desc 2", "open", 2, "task", nil, time.Now(), time.Now(), nil))

	filter := tickets.TicketFilter{Limit: 5}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(ticketList))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_CombinedFilters(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND status = \? AND issue_type = \? AND title LIKE \? ORDER BY priority ASC, updated_at DESC LIMIT \?`).
		WithArgs("open", "bug", "%crash%", 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-009", "Crash bug", "It crashes", "open", 0, "bug", nil, time.Now(), time.Now(), nil))

	filter := tickets.TicketFilter{
		Status:    "open",
		IssueType: "bug",
		Search:    "crash",
		Limit:     10,
	}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(ticketList))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_WithAssignee(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	assignee := "user@example.com"
	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 ORDER BY priority ASC, updated_at DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}).
			AddRow("bb-010", "Assigned Ticket", "Work", "open", 1, "task", assignee, time.Now(), time.Now(), nil))

	filter := tickets.TicketFilter{}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(ticketList))
	}

	if ticketList[0].Assignee != assignee {
		t.Errorf("expected assignee %q, got %q", assignee, ticketList[0].Assignee)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_EmptyResult(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 ORDER BY priority ASC, updated_at DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority", "issue_type", "assignee", "created_at", "updated_at", "parent_id",
		}))

	filter := tickets.TicketFilter{}
	ticketList, err := store.ListTickets(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ticketList) != 0 {
		t.Errorf("expected 0 tickets, got %d", len(ticketList))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_ListTickets_StoreClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	store := &ServerStore{
		db:     db,
		mode:   ServerMode,
		closed: true,
	}

	filter := tickets.TicketFilter{}
	_, err = store.ListTickets(context.Background(), filter)

	if err == nil {
		t.Fatal("expected error for closed store")
	}

	if err.Error() != "store is closed" {
		t.Errorf("expected 'store is closed' error, got: %v", err)
	}
}

func TestStore_ListTickets_NoDBConnection(t *testing.T) {
	store := &ServerStore{
		db:     nil,
		mode:   ServerMode,
		closed: false,
	}

	filter := tickets.TicketFilter{}
	_, err := store.ListTickets(context.Background(), filter)

	if err == nil {
		t.Fatal("expected error for nil db")
	}

	if err.Error() != "store has no database connection" {
		t.Errorf("expected 'store has no database connection' error, got: %v", err)
	}
}

func TestStore_LatestUpdate_Success(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(`SELECT MAX\(updated_at\) FROM ready_issues`).
		WillReturnRows(sqlmock.NewRows([]string{"MAX(updated_at)"}).
			AddRow(now))

	latest, err := store.LatestUpdate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !latest.Equal(now) {
		t.Errorf("expected %v, got %v", now, latest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_LatestUpdate_EmptyTable(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT MAX\(updated_at\) FROM ready_issues`).
		WillReturnRows(sqlmock.NewRows([]string{"MAX(updated_at)"}).
			AddRow(nil))

	latest, err := store.LatestUpdate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !latest.IsZero() {
		t.Errorf("expected zero time, got %v", latest)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_LatestUpdate_StoreClosed(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	store := &ServerStore{
		db:     db,
		mode:   ServerMode,
		closed: true,
	}

	_, err = store.LatestUpdate(context.Background())

	if err == nil {
		t.Fatal("expected error for closed store")
	}

	if err.Error() != "store is closed" {
		t.Errorf("expected 'store is closed' error, got: %v", err)
	}
}

func TestStore_LatestUpdate_NoDBConnection(t *testing.T) {
	store := &ServerStore{
		db:     nil,
		mode:   ServerMode,
		closed: false,
	}

	_, err := store.LatestUpdate(context.Background())

	if err == nil {
		t.Fatal("expected error for nil db")
	}

	if err.Error() != "store has no database connection" {
		t.Errorf("expected 'store has no database connection' error, got: %v", err)
	}
}

func TestBuildListTicketsQuery(t *testing.T) {
	tests := []struct {
		name     string
		filter   tickets.TicketFilter
		expected string
		args     []any
	}{
		{
			name:     "no filters",
			filter:   tickets.TicketFilter{},
			expected: "SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 ORDER BY priority ASC, updated_at DESC",
			args:     nil,
		},
		{
			name:     "status filter",
			filter:   tickets.TicketFilter{Status: "open"},
			expected: "SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND status = ? ORDER BY priority ASC, updated_at DESC",
			args:     []any{"open"},
		},
		{
			name:     "issue type filter",
			filter:   tickets.TicketFilter{IssueType: "bug"},
			expected: "SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND issue_type = ? ORDER BY priority ASC, updated_at DESC",
			args:     []any{"bug"},
		},
		{
			name:     "search filter",
			filter:   tickets.TicketFilter{Search: "test"},
			expected: "SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND title LIKE ? ORDER BY priority ASC, updated_at DESC",
			args:     []any{"%test%"},
		},
		{
			name:     "limit filter",
			filter:   tickets.TicketFilter{Limit: 10},
			expected: "SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 ORDER BY priority ASC, updated_at DESC LIMIT ?",
			args:     []any{10},
		},
		{
			name:     "combined filters",
			filter:   tickets.TicketFilter{Status: "open", IssueType: "feature", Search: "auth", Limit: 5},
			expected: "SELECT ready_issues.id, ready_issues.title, ready_issues.description, ready_issues.status, ready_issues.priority, ready_issues.issue_type, ready_issues.assignee, ready_issues.created_at, ready_issues.updated_at, d.depends_on_id AS parent_id FROM ready_issues LEFT JOIN dependencies d ON ready_issues.id = d.issue_id AND d.type = 'parent-child' WHERE 1=1 AND status = ? AND issue_type = ? AND title LIKE ? ORDER BY priority ASC, updated_at DESC LIMIT ?",
			args:     []any{"open", "feature", "%auth%", 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := buildListTicketsQuery(tt.filter)

			if query != tt.expected {
				t.Errorf("query mismatch\nexpected: %s\ngot:      %s", tt.expected, query)
			}

			if len(args) != len(tt.args) {
				t.Errorf("args length mismatch: expected %d, got %d", len(tt.args), len(args))
			}

			for i := range tt.args {
				if args[i] != tt.args[i] {
					t.Errorf("arg[%d] mismatch: expected %v, got %v", i, tt.args[i], args[i])
				}
			}
		})
	}
}

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
