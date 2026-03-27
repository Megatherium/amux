# UI Flow & View States

## Source Files

- `internal/ui/model.go` — UIModel, Init, message handlers
- `internal/ui/model_types.go` — UIModel struct, ViewState enum, FocusColumn enum
- `internal/ui/model_update.go` — Update() dispatch, key binding management
- `internal/ui/model_view.go` — View() rendering
- `internal/ui/handlers_enter.go` — Enter key handling per view state
- `internal/ui/handlers_navigation.go` — Focus/column navigation
- `internal/ui/view_matrix.go` — 4-column matrix view (Ticket/Harness/Model/Agent)
- `internal/ui/confirm.go` — Launch confirmation screen
- `internal/ui/view_state.go` — State management helpers
- `internal/ui/ticket_list.go` — Ticket list items
- `internal/ui/harness_select.go` — Harness list items
- `internal/ui/model_select.go` — Model list items
- `internal/ui/agent_select.go` — Agent list items
- `internal/ui/agents.go` — Agent sidebar management
- `internal/ui/sidebar.go` — Sidebar model
- `internal/ui/inline_edit.go` — Inline template editor
- `internal/ui/filepicker/` — Forked file picker with recents

## ViewState Machine

```
ViewStateLoading → ViewStateMatrix ←→ ViewStateConfirm
                      ↕                      ↕
              ViewStateFilePicker     ViewStateInlineEdit
                      ↕
              ViewStateAddProjectModal
                      
              ViewStateAgentOutput (sidebar agent view)
              ViewStateError (error display)
```

## Focus Columns

The matrix view has 5 focus columns:
```
FocusSidebar → FocusTickets → FocusHarness → FocusModel → FocusAgent
```

Each column is a `list.Model` from bubbles. The user navigates right (Enter)
to make a selection and advance focus.

## Selection Flow

```
1. User selects ticket (FocusTickets + Enter)
   → m.selection.Ticket = selected ticket
   → advance focus to FocusHarness

2. User selects harness (FocusHarness + Enter)
   → m.selection.Harness = selected harness
   → handleModelSkip() (auto-select if only 1 model)
   → handleAgentSkip() (auto-select if only 1 agent)
   → advance focus

3. User selects model (FocusModel + Enter)
   → m.selection.Model = selected model
   → handleAgentSkip()
   → advance focus

4. User selects agent (FocusAgent + Enter)
   → m.selection.Agent = selected agent
   → state = ViewStateConfirm
   → reloadTemplates() to show rendered command

5. Confirm screen shows rendered command/prompt
   → Enter = launch (state = ViewStateMatrix, launchCmd())
   → Esc = go back
   → 'e' = inline edit template
   → 'C' = file picker to load template from file
```

## Launch Flow

```go
func (m UIModel) launchCmd() tea.Cmd {
    return func() tea.Msg {
        // 1. Build template context from selection
        ctx := config.BuildTemplateContext(m.selection, workDir)
        
        // 2. Render prompt first (so command can use {{.Prompt}})
        renderedPrompt, _ := renderer.RenderPrompt(harness, ctx)
        ctx.Prompt = renderedPrompt
        
        // 3. Render command
        renderedCmd, _ := renderer.RenderCommand(harness, ctx)
        
        // 4. Create LaunchSpec
        spec := domain.LaunchSpec{...}
        
        // 5. Launch via tmux
        result, _ := launcher.Launch(ctx, spec)
        
        // 6. Persist running agent to Dolt
        store.UpsertRunningAgent(...)
        
        return launchResultMsg{...}
    }
}
```

## Amux Equivalent

**The entire matrix UI is discarded.** Amux uses a different flow:

### Option A: Multi-Step Dialog

Extend amux's `common.Dialog` to create a multi-step tab creation flow:

```
1. "Select Ticket" dialog — lists beads issues
2. "Select Assistant" dialog — amux's existing assistant picker
3. "Select Model" dialog — resolved from discovery registry
4. "Select Agent Mode" dialog — coder/researcher/etc
5. "Confirm & Launch" — shows rendered command
```

This maps to amux's dialog system with `DialogSelectAssistant` already existing.

### Option B: Dashboard Integration

Add tickets as nodes in the dashboard tree:

```
[Project: blunderbust]
  [Worktree: main]
    [Ticket: bb-123] ← new node type
      → sidebar shows ticket details
      → Enter creates new tab with ticket context
  [Worktree: feature-x]
    [Ticket: bb-456]
```

Selecting a ticket node + pressing Enter could trigger a dialog for
assistant/model/agent selection, then create the tab.

### Option C: Prefix Command

Use amux's prefix palette (leader key system) for ticket-aware tab creation:
- `prefix + t` → "New ticket tab" → selection flow
- `prefix + T` → "Quick ticket launch" → use defaults

## Key Bindings to Port

| Blunderbust Key | Action | Amux Equivalent |
|---|---|---|
| `Enter` | Confirm selection / advance focus | Dialog confirmation |
| `Esc` / `Back` | Go back / cancel | Dialog cancel |
| `Tab` | Swap file picker panes | N/A (amux file picker) |
| `a` (file picker) | Select directory as project | Amux add project |
| `e` (confirm) | Inline edit template | Could be useful |
| `C` (confirm) | Pick template from file | File picker integration |
| `r` (tickets) | Refresh ticket list | Background refresh |
| `p` | Open file picker | Amux file picker |
| `s` | Toggle sidebar | Amux sidebar toggle |

## Agent Monitoring

Blunderbust polls tmux window status every few seconds and reads pane output
for agent monitoring. Amux does this natively via:
- `app_tmux_activity.go` — hysteresis-based activity tracking
- PTY reader → vterm pipeline — live output rendering

The only addition needed is associating ticket metadata with tabs so the sidebar
can show "working on bb-123: Fix login bug".

## Ticket Auto-Refresh

Blunderbust polls tickets every 3 seconds:
```go
tea.Tick(ticketPollingInterval, func(t time.Time) tea.Msg {
    return ticketUpdateCheckMsg{}
})
```

If `LatestUpdate()` returns a newer timestamp, it refreshes the ticket list.
This can be adapted to amux's state watcher pattern.

## Inline Template Editor

Blunderbust has a textarea-based inline editor for templates on the confirm
screen. Users can edit `command_template` or `prompt_template` before launching.
Changes are temporary (not saved to config).

**Amux approach:** This could be a dialog or a temporary tab in the center pane.
