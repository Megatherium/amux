package cli

import (
	"sort"
	"strconv"
	"strings"

	"github.com/andyrewlee/amux/internal/tmux"
)

type taskAgentLookupMode int

const (
	taskAgentLookupForStart taskAgentLookupMode = iota
	taskAgentLookupForStatus
)

func findLatestTaskAgentSnapshot(opts tmux.Options, workspaceID, assistant string, mode taskAgentLookupMode) (*taskAgentCandidate, taskAgentSnapshot, error) {
	candidates, err := listTaskAgentCandidates(opts, workspaceID, assistant)
	if err != nil {
		return nil, taskAgentSnapshot{}, err
	}
	if len(candidates) == 0 {
		return nil, taskAgentSnapshot{}, nil
	}

	selected := selectTaskAgentCandidate(candidates, strings.TrimSpace(assistant) != "", mode, opts)
	if selected == nil {
		return nil, taskAgentSnapshot{}, nil
	}
	candidate := selected.candidate
	return &candidate, selected.snap, nil
}

func listTaskAgentCandidates(opts tmux.Options, workspaceID, assistant string) ([]taskAgentCandidate, error) {
	rows, err := tmuxSessionsWithTags(
		map[string]string{
			"@amux":           "1",
			"@amux_workspace": workspaceID,
			"@amux_type":      "agent",
		},
		[]string{"@amux_tab", "@amux_assistant", "@amux_created_at", tmux.TagLastOutputAt},
		opts,
	)
	if err != nil {
		return nil, err
	}

	out := make([]taskAgentCandidate, 0, len(rows))
	assistant = strings.ToLower(strings.TrimSpace(assistant))
	for _, row := range rows {
		tagAssistant := strings.ToLower(strings.TrimSpace(row.Tags["@amux_assistant"]))
		if assistant != "" && tagAssistant != "" && tagAssistant != assistant {
			continue
		}

		tabID := strings.TrimSpace(row.Tags["@amux_tab"])
		if tabID == "" {
			tabID = inferTabIDFromSessionName(row.Name, workspaceID)
		}
		agentID := formatAgentID(workspaceID, tabID)
		if strings.TrimSpace(agentID) == "" {
			agentID = workspaceID + ":" + row.Name
		}

		createdAt := int64(0)
		if raw := strings.TrimSpace(row.Tags["@amux_created_at"]); raw != "" {
			if ts, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil {
				createdAt = ts
			}
		}
		lastOutputAt, hasLastOutput := parseSessionTagTime(row.Tags[tmux.TagLastOutputAt])
		candidate := taskAgentCandidate{
			SessionName:     row.Name,
			AgentID:         agentID,
			Assistant:       nonEmpty(tagAssistant, assistant),
			HasAssistantTag: tagAssistant != "",
			CreatedAt:       createdAt,
			LastOutputAt:    lastOutputAt,
			HasLastOutput:   hasLastOutput,
		}
		out = append(out, candidate)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].SessionName > out[j].SessionName
	})
	return out, nil
}

type taskObservedAgentCandidate struct {
	candidate taskAgentCandidate
	snap      taskAgentSnapshot
	status    taskObservedAgentStatus
}

type taskObservedAgentStatus int

const (
	taskObservedAgentStatusActive taskObservedAgentStatus = iota
	taskObservedAgentStatusCompleted
	taskObservedAgentStatusExited
	taskObservedAgentStatusUncertain
)

func selectTaskAgentCandidate(candidates []taskAgentCandidate, hasAssistantFilter bool, mode taskAgentLookupMode, opts tmux.Options) *taskObservedAgentCandidate {
	if !hasAssistantFilter {
		if mode == taskAgentLookupForStart {
			return selectTaskStartCandidateGroup(candidates, opts)
		}
		return selectTaskStatusCandidateGroup(candidates, opts)
	}

	exact := make([]taskAgentCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.HasAssistantTag {
			exact = append(exact, candidate)
		}
	}

	if mode == taskAgentLookupForStart {
		return selectTaskStartCandidateGroup(exact, opts)
	}
	return selectTaskStatusCandidateGroup(exact, opts)
}

func selectTaskStartCandidateGroup(candidates []taskAgentCandidate, opts tmux.Options) *taskObservedAgentCandidate {
	if len(candidates) == 0 {
		return nil
	}

	var completedFallback *taskObservedAgentCandidate
	for _, candidate := range candidates {
		observed := observeTaskAgentCandidate(candidate, opts)
		switch observed.status {
		case taskObservedAgentStatusActive:
			return &observed
		case taskObservedAgentStatusUncertain:
			return &observed
		case taskObservedAgentStatusCompleted:
			if completedFallback == nil {
				completedFallback = &observed
			}
		}
	}
	return completedFallback
}

func selectTaskStatusCandidateGroup(candidates []taskAgentCandidate, opts tmux.Options) *taskObservedAgentCandidate {
	if len(candidates) == 0 {
		return nil
	}

	var terminalFallback *taskObservedAgentCandidate
	for _, candidate := range candidates {
		observed := observeTaskAgentCandidate(candidate, opts)
		switch observed.status {
		case taskObservedAgentStatusActive:
			return &observed
		case taskObservedAgentStatusUncertain:
			return &observed
		case taskObservedAgentStatusCompleted, taskObservedAgentStatusExited:
			if terminalFallback == nil {
				terminalFallback = &observed
			}
		}
	}
	return terminalFallback
}

func observeTaskAgentCandidate(candidate taskAgentCandidate, opts tmux.Options) taskObservedAgentCandidate {
	state, stateErr := tmuxSessionStateFor(candidate.SessionName, opts)
	if stateErr == nil && (!state.Exists || !state.HasLivePane) {
		snap := taskAgentSnapshot{
			Summary:       "(no visible output yet)",
			LatestLine:    "(no visible output yet)",
			SessionExited: true,
		}
		return taskObservedAgentCandidate{
			candidate: candidate,
			snap:      snap,
			status:    taskObservedAgentStatusExited,
		}
	}

	snap, captured := captureTaskAgentSnapshot(candidate.SessionName, opts)
	status := taskObservedAgentStatusActive
	if !captured {
		status = taskObservedAgentStatusUncertain
	} else if taskStatusLooksComplete(candidate, snap) {
		status = taskObservedAgentStatusCompleted
	}
	return taskObservedAgentCandidate{
		candidate: candidate,
		snap:      snap,
		status:    status,
	}
}

func captureTaskAgentSnapshot(sessionName string, opts tmux.Options) (taskAgentSnapshot, bool) {
	content, ok := captureAgentPaneWithRetry(sessionName, taskCaptureLines, opts)
	if !ok {
		return taskAgentSnapshot{
			Summary:    "(no visible output yet)",
			LatestLine: "(no visible output yet)",
		}, false
	}

	compact := strings.TrimSpace(compactAgentOutput(content))
	if compact == "" {
		compact = strings.TrimSpace(content)
	}
	latest := strings.TrimSpace(lastNonEmptyLine(compact))
	if latest == "" {
		latest = "(no visible output yet)"
	}
	needsInput, inputHint := detectNeedsInput(compact)
	if !needsInput {
		needsInput, inputHint = detectNeedsInput(content)
	}
	summary := summarizeWaitResponse("idle", latest, needsInput, inputHint)
	if strings.TrimSpace(summary) == "" {
		summary = latest
	}

	return taskAgentSnapshot{
		Summary:    summary,
		LatestLine: latest,
		NeedsInput: needsInput,
		InputHint:  strings.TrimSpace(inputHint),
	}, true
}

func inferTabIDFromSessionName(sessionName, workspaceID string) string {
	prefix := tmux.SessionName("amux", workspaceID) + "-"
	if strings.HasPrefix(sessionName, prefix) {
		return strings.TrimPrefix(sessionName, prefix)
	}
	return ""
}
