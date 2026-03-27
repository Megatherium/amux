# bmx-qia: Port Dolt store (TicketStore implementation) — PICKUP PLAN

## Status
- Issue: `bmx-qia` (set to in_progress)
- All 3 blocking dependencies closed: bmx-1ed ✅, bmx-4d8 ✅, bmx-6ha ✅

## What Was Done
1. **`server.go`** — CORRECTED. Added `Mode` type + `ServerMode` const, and fixed the port-assignment bug:
   - Added `type Mode string` and `const ServerMode Mode = "server"` near top
   - Added `if resolvedPort > 0 { metadata.ServerPort = resolvedPort }` in `newServerStore()` after line 65
   - This was the bug: resolvedPort was computed but never assigned back to metadata.ServerPort

## What Remains

### 1. Verify `server.go` is correct
The bash `cat > ...` write worked (no error). Check:
```bash
head -30 /home/sloth/Documents/projects/amux/internal/tickets/dolt/server.go
# Should show: type Mode string, const ServerMode Mode = "server"
grep -n "metadata.ServerPort = resolvedPort" /home/sloth/Documents/projects/amux/internal/tickets/dolt/server.go
# Should show the assignment line
```

### 2. Create `store.go` 
A `cat > ...` command was run but let it run in background. CHECK if it exists:
```bash
ls -la /home/sloth/Documents/projects/amux/internal/tickets/dolt/store.go
cat /home/sloth/Documents/projects/amux/internal/tickets/dolt/store.go
```

If it doesn't exist or is empty, recreate it. The content should be:

**`internal/tickets/dolt/store.go`** — Port from `blunderbust/internal/data/dolt/store.go`:
- `Store` struct: embeds nothing (unlike blunderbuss's `ServerStore` embedding); has `serverStore *ServerStore`, `mode Mode`, `closed bool`, `beadsDir string`, `metadata *Metadata`
- Compile-time interface check: `var _ tickets.TicketStore = (*Store)(nil)`
- `ErrServerNotRunningStore` + `IsErrServerNotRunningStore()` — distinct from `ErrServerNotRunning` in server.go
- `handleServerMode()` — handles connection errors, autostart, retry logic
- `newServerStoreWithMode()` — helper to create Store with ServerMode
- `NewStore(ctx, beadsDir, autostart)` — loads metadata, calls handleServerMode
- `IsConnectionError(err)` — detects connection failure strings
- `Close()`, `DB()`, `CanRetryConnection()`, `AutostartEnabled()` — delegation to serverStore
- `TryStartServer(ctx)` — restarts server and reconnects
- `ListTickets(ctx, filter)` — queries `ready_issues`, uses `buildListTicketsQuery` + `scanTickets`
- `LatestUpdate(ctx)` — `SELECT MAX(updated_at) FROM ready_issues`
- `buildListTicketsQuery(filter)` — parameterized SQL builder
- `scanTickets(rows)` — scans sql.Rows into `[]tickets.Ticket`

### 3. Write `store_test.go`
**`internal/tickets/dolt/store_test.go`**:
- `TestStore_ImplementsTicketStore` — compile-time check + `ListTickets` + `LatestUpdate` on fake
- `TestNewStore_InvalidBeadsDir` — non-existent beads dir returns error
- `TestBuildListTicketsQuery` — query string construction with all filter combos
- `TestScanTickets` — null assignee, valid rows, error row

### 4. Build & Test
```bash
go build ./internal/tickets/dolt/
go test ./internal/tickets/dolt/ -v -run "Store|List"
make lint-strict-new   # or check available lint commands
```

### 5. Review Protocol
After implementation is verified working:
- Run CODE REVIEW & REFINEMENT PROTOCOL
- Create review ticket: `bd create --title="Review: bmx-qia Port Dolt store" --type=review`
- Do NOT close bmx-qia until review is done

## Key Context
- **Source**: `https://raw.githubusercontent.com/Megatherium/blunderbust/master/internal/data/dolt/store.go`
- **Interface**: `tickets.TicketStore` from `internal/tickets/store.go`
- **Types**: `tickets.Ticket`, `tickets.TicketFilter` from `internal/tickets/types.go`
- **ServerStore already exists** in `server.go` with: `newServerStore()`, `buildServerDSN()`, `StartServer()`, `TryStartServer()`, `IsErrServerNotRunning()`
- **Bug fix** (already applied): port resolution was not being assigned back to `metadata.ServerPort`
- **go-sql-driver/mysql**: already in go.mod ✅
