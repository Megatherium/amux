package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andyrewlee/amux/internal/tmux"
)

// === Leader-election protocol over tmux global options ===
//
// Multiple amux instances may share a single tmux server. Activity scanning
// (list-sessions, capture-pane, etc.) is expensive, so only one instance
// should run it. This file implements a cooperative leader-election protocol
// using tmux global options (@amux_activity_*) as a shared store.
//
// Roles:
//   - Owner — the elected scanner. Performs local activity scans and
//     periodically publishes an active-workspace snapshot that followers
//     consume instead of running their own scans.
//   - Follower — skips the local scan and applies the owner's snapshot,
//     falling back to a local scan when the snapshot is stale or missing.
//
// Lease mechanism:
//   - @amux_activity_owner       — owner instance ID (string)
//   - @amux_activity_owner_heartbeat_ms — last heartbeat (unix millis)
//   - @amux_activity_owner_epoch      — monotonic epoch for tombstoning stale owners
//
// Tmux global options have no atomic compare-and-swap. Ownership is claimed
// by write-then-confirm-read: write a lease, re-read it, and if the stored
// owner/epoch don't match our write we lost the race. Epoch numbers prevent
// split-brain: followers reject snapshots whose epoch doesn't match the
// current lease epoch.
//
// Snapshot format: "epoch;timestamp_ms;payload"
//   - epoch       — owner's lease epoch at publish time
//   - timestamp   — when the snapshot was created (unix millis)
//   - payload     — "j:" + base64(json) of sorted workspace IDs
//
// Legacy payloads (no "j:" prefix) are comma-delimited workspace IDs with
// optional "b:" base64 encoding. These are decoded for backward compatibility.
//
// resolveTmuxActivityScanRole path summary:
//   1. No lease (error)      → become owner (epoch=0, caller normalizes)
//   2. Alive lease, other ID → follower: read snapshot, apply if fresh
//   3. Alive lease, our ID   → owner: keep existing epoch
//   4. Expired lease         → claim with epoch+1, confirm-read
//   5. Confirmed claim       → owner; otherwise → follower of winner

const (
	tmuxActivityOwnerOption     = "@amux_activity_owner"
	tmuxActivityHeartbeatOption = "@amux_activity_owner_heartbeat_ms"
	tmuxActivityEpochOption     = "@amux_activity_owner_epoch"
	tmuxActivitySnapshotOption  = "@amux_activity_active_workspaces"
)

var errTmuxActivityOwnershipLostAfterPublish = errors.New("tmux activity ownership lost after snapshot publish")

type tmuxActivityRole int

const (
	tmuxActivityRoleOwner tmuxActivityRole = iota
	tmuxActivityRoleFollower
)

// activityScanRoleResult bundles the return values of resolveTmuxActivityScanRole.
// sharedActive and applyShared are meaningful only when role == tmuxActivityRoleFollower.
type activityScanRoleResult struct {
	role         tmuxActivityRole
	sharedActive map[string]bool
	applyShared  bool
	epoch        int64
	err          error
}

func (a *App) sharedTmuxActivityEnabled() bool {
	return strings.TrimSpace(a.instanceID) != ""
}

func (a *App) resolveTmuxActivityScanRole(
	opts tmux.Options,
	now time.Time,
) activityScanRoleResult {
	// instanceID is assigned once at init; trim once here so all lease-owner
	// comparisons use the same normalized representation.
	instanceID := strings.TrimSpace(a.instanceID)
	lease, err := readTmuxActivityOwnerLease(opts)
	if err != nil {
		// Epoch 0 is intentional on unresolved ownership; callers normalize to 1
		// only when publishing as owner in a known epoch.
		return activityScanRoleResult{role: tmuxActivityRoleOwner, err: err}
	}
	if ownerLeaseAlive(lease, now) && lease.ownerID != instanceID {
		active, ok, err := readTmuxActivitySnapshot(opts, now, lease.epoch)
		if err != nil {
			return activityScanRoleResult{role: tmuxActivityRoleFollower, epoch: lease.epoch, err: err}
		}
		return activityScanRoleResult{role: tmuxActivityRoleFollower, sharedActive: active, applyShared: ok, epoch: lease.epoch}
	}
	if ownerLeaseAlive(lease, now) && lease.ownerID == instanceID {
		epoch := lease.epoch
		if epoch < 1 {
			epoch = 1
		}
		return activityScanRoleResult{role: tmuxActivityRoleOwner, epoch: epoch}
	}

	candidateEpoch := lease.epoch + 1
	if candidateEpoch < 1 {
		candidateEpoch = 1
	}
	// tmux global options provide no atomic compare-and-swap primitive. Claim by
	// write-then-confirm-read and rely on epoch checks to prevent split-brain use.
	if err := writeTmuxActivityOwnerLease(opts, instanceID, candidateEpoch, now); err != nil {
		return activityScanRoleResult{role: tmuxActivityRoleOwner, epoch: candidateEpoch, err: err}
	}
	confirmedLease, err := readTmuxActivityOwnerLease(opts)
	if err != nil {
		return activityScanRoleResult{role: tmuxActivityRoleOwner, epoch: candidateEpoch, err: err}
	}
	if confirmedLease.ownerID != instanceID || confirmedLease.epoch != candidateEpoch {
		active, ok, err := readTmuxActivitySnapshot(opts, now, confirmedLease.epoch)
		if err != nil {
			return activityScanRoleResult{role: tmuxActivityRoleFollower, epoch: confirmedLease.epoch, err: err}
		}
		return activityScanRoleResult{role: tmuxActivityRoleFollower, sharedActive: active, applyShared: ok, epoch: confirmedLease.epoch}
	}
	return activityScanRoleResult{role: tmuxActivityRoleOwner, epoch: candidateEpoch}
}

func (a *App) canPublishTmuxActivitySnapshot(opts tmux.Options, epoch int64, now time.Time) (bool, int64, error) {
	instanceID := strings.TrimSpace(a.instanceID)
	if instanceID == "" {
		return false, 0, nil
	}
	lease, err := readTmuxActivityOwnerLease(opts)
	if err != nil {
		return false, 0, err
	}
	if !ownerLeaseAlive(lease, now) {
		return false, lease.epoch, nil
	}
	if lease.ownerID != instanceID || lease.epoch != epoch {
		return false, lease.epoch, nil
	}
	return true, lease.epoch, nil
}

func (a *App) publishTmuxActivitySnapshot(opts tmux.Options, active map[string]bool, epoch int64, now time.Time) error {
	if err := tmux.SetGlobalOptionValue(tmuxActivitySnapshotOption, encodeTmuxActivitySnapshot(active, epoch, now), opts); err != nil {
		return err
	}
	// Snapshot write and ownership validation are not atomic; epoch checks on
	// reads ensure followers ignore snapshots from superseded owners.
	canPublish, _, err := a.canPublishTmuxActivitySnapshot(opts, epoch, time.Now())
	if err != nil {
		return err
	}
	if !canPublish {
		return errTmuxActivityOwnershipLostAfterPublish
	}
	// Heartbeat renewal may race with ownership turnover. Ownership/epoch checks
	// on readers and subsequent scans handle this by treating mismatches as stale.
	return renewTmuxActivityOwnerLeaseHeartbeat(opts, now)
}

type tmuxActivityLease struct {
	ownerID     string
	heartbeatAt time.Time
	epoch       int64
}

func ownerLeaseAlive(lease tmuxActivityLease, now time.Time) bool {
	if strings.TrimSpace(lease.ownerID) == "" {
		return false
	}
	if lease.heartbeatAt.IsZero() {
		return false
	}
	if lease.heartbeatAt.After(now) {
		// Small forward skew is tolerated, but large future timestamps should not
		// keep stale ownership alive indefinitely.
		return lease.heartbeatAt.Sub(now) <= tmuxActivityOwnerFutureSkewTolerance
	}
	return now.Sub(lease.heartbeatAt) <= tmuxActivityOwnerLeaseTTL
}

func readTmuxActivityOwnerLease(opts tmux.Options) (tmuxActivityLease, error) {
	lease := tmuxActivityLease{}
	values, err := tmux.GlobalOptionValues([]string{
		tmuxActivityOwnerOption,
		tmuxActivityHeartbeatOption,
		tmuxActivityEpochOption,
	}, opts)
	if err != nil {
		return lease, err
	}
	lease.ownerID = strings.TrimSpace(values[tmuxActivityOwnerOption])

	heartbeatRaw := strings.TrimSpace(values[tmuxActivityHeartbeatOption])
	if heartbeatRaw != "" {
		heartbeatMS, parseErr := strconv.ParseInt(heartbeatRaw, 10, 64)
		if parseErr == nil && heartbeatMS > 0 {
			lease.heartbeatAt = time.UnixMilli(heartbeatMS)
		}
	}

	epochRaw := strings.TrimSpace(values[tmuxActivityEpochOption])
	if epochRaw != "" {
		epoch, parseErr := strconv.ParseInt(epochRaw, 10, 64)
		if parseErr == nil && epoch > 0 {
			lease.epoch = epoch
		}
	}
	return lease, nil
}

func writeTmuxActivityOwnerLease(opts tmux.Options, ownerID string, epoch int64, now time.Time) error {
	if epoch < 1 {
		epoch = 1
	}
	return tmux.SetGlobalOptionValues([]tmux.OptionValue{
		{Key: tmuxActivityOwnerOption, Value: strings.TrimSpace(ownerID)},
		{Key: tmuxActivityEpochOption, Value: strconv.FormatInt(epoch, 10)},
		{Key: tmuxActivityHeartbeatOption, Value: strconv.FormatInt(now.UnixMilli(), 10)},
	}, opts)
}

func renewTmuxActivityOwnerLeaseHeartbeat(opts tmux.Options, now time.Time) error {
	return tmux.SetGlobalOptionValue(tmuxActivityHeartbeatOption, strconv.FormatInt(now.UnixMilli(), 10), opts)
}

func readTmuxActivitySnapshot(opts tmux.Options, now time.Time, expectedEpoch int64) (map[string]bool, bool, error) {
	raw, err := tmux.GlobalOptionValue(tmuxActivitySnapshotOption, opts)
	if err != nil {
		return nil, false, err
	}
	parsed, snapshotEpoch, at, ok := decodeTmuxActivitySnapshot(raw)
	if !ok {
		return nil, false, nil
	}
	if expectedEpoch > 0 && snapshotEpoch != expectedEpoch {
		return nil, false, nil
	}
	if at.After(now) {
		return parsed, true, nil
	}
	if now.Sub(at) > tmuxActivitySnapshotStaleAfter {
		return nil, false, nil
	}
	return parsed, true, nil
}

func encodeTmuxActivitySnapshot(active map[string]bool, epoch int64, now time.Time) string {
	if epoch < 1 {
		epoch = 1
	}
	ids := make([]string, 0, len(active))
	for wsID, isActive := range active {
		if isActive {
			trimmed := strings.TrimSpace(wsID)
			if trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
	}
	sort.Strings(ids)
	encodedPayload, err := json.Marshal(ids)
	if err != nil {
		encodedPayload = []byte("[]")
	}
	payload := "j:" + base64.RawURLEncoding.EncodeToString(encodedPayload)
	return strconv.FormatInt(epoch, 10) + ";" + strconv.FormatInt(now.UnixMilli(), 10) + ";" + payload
}

func decodeTmuxActivitySnapshot(raw string) (map[string]bool, int64, time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, 0, time.Time{}, false
	}
	parts := strings.SplitN(raw, ";", 3)
	if len(parts) != 3 {
		return nil, 0, time.Time{}, false
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || epoch <= 0 {
		return nil, 0, time.Time{}, false
	}
	timestampMS, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || timestampMS <= 0 {
		return nil, 0, time.Time{}, false
	}
	active := make(map[string]bool)
	payload := strings.TrimSpace(parts[2])
	if payload == "" {
		return active, epoch, time.UnixMilli(timestampMS), true
	}
	if strings.HasPrefix(payload, "j:") {
		decodedPayload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(payload, "j:"))
		if err == nil {
			var ids []string
			if err := json.Unmarshal(decodedPayload, &ids); err == nil {
				for _, candidate := range ids {
					wsID := strings.TrimSpace(candidate)
					if wsID == "" {
						continue
					}
					active[wsID] = true
				}
				return active, epoch, time.UnixMilli(timestampMS), true
			}
		}
	}

	legacyCandidates := make([]string, 0)
	for _, candidate := range strings.Split(payload, ",") {
		wsID := strings.TrimSpace(candidate)
		if wsID != "" {
			legacyCandidates = append(legacyCandidates, wsID)
		}
	}
	if len(legacyCandidates) == 0 {
		return active, epoch, time.UnixMilli(timestampMS), true
	}

	// Legacy payloads: comma-delimited plain IDs with optional b:<base64(id)> entries.
	// Note: plain IDs that literally start with "b:" and are valid base64 will be
	// interpreted as encoded legacy IDs by design for backward compatibility.
	for _, candidate := range legacyCandidates {
		if !strings.HasPrefix(candidate, "b:") {
			active[candidate] = true
			continue
		}

		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(candidate, "b:"))
		if err != nil {
			active[candidate] = true
			continue
		}

		wsID := strings.TrimSpace(string(decoded))
		if wsID == "" {
			active[candidate] = true
			continue
		}
		active[wsID] = true
	}
	return active, epoch, time.UnixMilli(timestampMS), true
}
