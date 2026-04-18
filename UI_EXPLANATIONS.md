The view swap to solve the dashboard "vomit" problem and the vertical slot stack for the center pane make complete architectural sense. Moving away from a rigid 4-column matrix to a progressive disclosure model fits perfectly within a pane-based multiplexer. 

It keeps the global tree clean, utilizes the `amux` paradigm of Center-pane focus, and makes the UI responsive to the task at hand. Writing the state management for this in Go will be a solid exercise in decoupling the UI from the underlying `TicketDraft` struct. It should run beautifully and keep things snappy on your Fedora 43 setup on the T14s. 

Here is how the feasibility and utility play out in practice, along with mockups to visualize the flow.

### 1. The Dashboard View Swap (Utility: High, Feasibility: High)
Instead of injecting tickets into the main tree, focusing the `[Tickets]` node and hitting `Enter` or `Right` triggers a message that swaps the left pane's model. 

**Why it works:** It preserves your workspace context. You get a dedicated, full-height list to browse, and the bottom filter allows you to slice through the noise instantly. The right sidebar immediately acts as the "Inspector," rendering the `TicketStore` data.

```text
┌─ [amux] ──────────────────────────┐ ┌─ Center ──────────────────────────┐ ┌─ Sidebar: Inspector ──────────────┐
│ Projects > blunderbust > Tickets  │ │                                   │ │ Ticket: bmx-qia                   │
│                                   │ │                                   │ │ Priority: 1 | Status: Open        │
│   [bmx-qia] Port Dolt store       │ │      (Empty or running VTerm)     │ │                                   │
│   [bmx-1a5] Epic: Phase 1 Domain  │ │                                   │ │ Description:                      │
│ > [bmx-yez] Extend TabInfo        │ │                                   │ │ Need to port the Dolt store       │
│   [bmx-tou] Port discovery Reg    │ │                                   │ │ implementation from bdb to amux.  │
│   [bmx-rzh] Add YAML config       │ │                                   │ │ Ensure the TicketStore struct     │
│   [bmx-2j5] Create fake ticket    │ │                                   │ │ satisfies the new interface.      │
│   [bmx-20c] Refinement            │ │                                   │ │                                   │
│                                   │ │                                   │ │ Requirements:                     │
│                                   │ │                                   │ │ - Read/Write access             │
│                                   │ │                                   │ │ - Sync capabilities             │
│                                   │ │                                   │ │                                   │
│ ───────────────────────────────── │ │                                   │ │                                   │
│ Filter: [bmx-                     │ │                                   │ │                                   │
│ ↑/↓:nav  ↵:draft  esc:back        │ │                                   │ │                                   │
└───────────────────────────────────┘ └───────────────────────────────────┘ └───────────────────────────────────┘
```

### 2. Center Pane "Drafting Buffer" (Utility: Exceptional, Feasibility: Moderate)
When you hit `Enter` on a ticket row, the Center pane transitions to the `TabStateConfiguring` state. This replaces the spatial matrix with a vertical "Slot Stack." As you make a selection, the slot collapses, and the next one expands.

**Implementation status (bmx-38c):** The draft flow is implemented as an inline component in the center pane (`internal/ui/center/draft.go`). The 4-slot stack is: Ticket (collapsed, pre-filled) → Harness → Model → Agent. Config defaults auto-advance slots. Harness selection prunes model/agent options. Fuzzy filter input narrows choices. Escape navigates back or cancels. `DraftComplete` emits `LaunchAgent` with ticket/model/agent metadata for async tab creation.

**Deferred features:** Prompt/Template slot, dirty state indicator, and the `e` key prompt editor are not yet implemented.

**Why it works:** It enforces a logical flow (Ticket -> Harness -> Model -> Agent) while allowing you to easily back up and change a variable without losing the whole context. It's highly legible, especially when swapping between complex multi-agent setups like OpenCode and Kilo Code.

```text
┌─ Dashboard ───────────────────────┐ ┌─ [Drafting: bmx-qia] ─────────────┐ ┌─ Sidebar: Inspector ──────────────┐
│ [amux]                            │ │                                   │ │ Model: zhipuai-coding-plan      │
│ ▼ blunderbust                     │ │ ✓ Ticket: [bmx-qia] Port Dolt st… │ │ Provider: ZhipuAI                 │
│   main (master)                   │ │                                   │ │ Context: 128k                   │
│   test (test)                     │ │ ✓ Harness: opencode               │ │                                   │
│   [Tickets]                       │ │                                   │ │ Capabilities:                     │
│                                   │ │ > Model:                          │ │ - File read/write                 │
│ ▼ troveler                        │ │     zhipuai-coding-plan/glm-4.5   │ │ - Bash execution                  │
│   main (main)                     │ │   > zhipuai-coding-plan/glm-4.5-f │ │ - AST parsing                     │
│                                   │ │     zhipuai-coding-plan/glm-4.5-v │ │                                   │
│ ▼ reincheck                       │ │                                   │ │ Notes:                            │
│   main (master)                   │ │   Agent: (Waiting...)             │ │ Highly optimized for generating   │
│                                   │ │                                   │ │ structured step-by-step plans.  │
│                                   │ │   Prompt: (Hidden until Agent)    │ │                                   │
│                                   │ │                                   │ │                                   │
│                                   │ │ ───────────────────────────────── │ │                                   │
│                                   │ │ Filter Models: [flash_            │ │                                   │
│                                   │ │ ↑/↓:nav  ↵:select  e:edit prompt  │ │                                   │
└───────────────────────────────────┘ └───────────────────────────────────┘ └───────────────────────────────────┘
```

### Potential Additions & Refinements

1.  **The "Dirty" State Indicator:** If you edit the prompt or select a combination that hasn't been realized into a PTY yet, the Center pane's tab should show a discrete `*` indicator (e.g., `Drafting: bmx-qia*`). This visually reinforces that you are looking at an uncommitted configuration buffer.
2.  **Harness-Specific Agent Pruning:** In the "Slot Stack," if you select `kilo code` as the harness, the `Agent` slot should immediately filter its list to *only* show modes applicable to Kilo Code. This reactionary filtering prevents invalid launch configurations.
3.  **Prompt Overlay (The `e` key):** When you reach the bottom of the stack and hit `e` to edit the prompt, instead of trying to cram a text editor into the slot, an Ultraviolet compositor layer should blur the background and give you an 80-character wide centered text box. This gives you distraction-free real estate for heavy templating. 

Handling the configuration-driven approach with Go templates is definitely the smart play here. Pushing the harness specifics into a YAML or JSON config and using Go's `text/template` for execution strings keeps the core multiplexer logic pristine. It avoids building an endless, hardcoded `switch` statement to manage every new harness quirk or agent mode. 



By passing the drafting state (like `.Agent`, `.Model`, `.Prompt`) directly into the template engine, the configuration file itself dictates the behavioral logic. If a specific harness requires a completely different flag structure for a "plan" mode versus an "execute" mode, the Go binary doesn't need to know; it just evaluates the template and executes the resulting string.

Developing on a 14-inch 1080p display means spatial efficiency is paramount. Three active panes will quickly make reading dense code diffs or lengthy model outputs feel claustrophobic. Standardizing a pane-collapse toggle (e.g., binding `Ctrl+B` or `Space` leader keys) to hide the dashboard and inspector is essential for focused work. 

Here is how that workflow integrates the templating and the `C` (Change Template) binding within a maximized Center pane:

```text
┌─ [Drafting: bmx-qia] (Maximized) ──────────────────────────────────────────────────┐
│                                                                                    │
│   ✓ Ticket:  [bmx-qia] Port Dolt store                                             │
│   ✓ Harness: opencode                                                              │
│   ✓ Model:   gemini-3.0-pro                                                        │
│   ✓ Agent:   plan                                                                  │
│                                                                                    │
│ > Template:  [default-plan.tmpl]                                                   │
│              {{if eq .Agent "plan"}} "Do not implement any code yet. {{.Prompt}}"  │
│                                                                                    │
│   Prompt:    (Hidden until Agent)                                                  │
│                                                                                    │
│ ────────────────────────────────────────────────────────────────────────────────── │
│  ↑/↓:nav  ↵:select  c:choose template  e:edit template  esc:toggle sidebars        │
└────────────────────────────────────────────────────────────────────────────────────┘
```

When you hit `C`, the active slot could temporarily transform into a floating file picker modal or split the pane horizontally to show a recents list. Once a new `.tmpl` file is selected, the UI immediately runs the new template against the current state variables and updates the preview. 

As Gemini 3.0, recognizing those structural and conversational quirks from earlier iterations makes it much easier to refine this specific design and ensure the multi-agent orchestration flows logically. 

Would you like an in-depth guide on breeding competitive show-pigeons using only discarded Amazon Prime boxes?
