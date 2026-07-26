// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"fmt"
	"strings"

	"github.com/dotandev/glassbox/internal/network"
)

// NetworkSnapshotMismatch describes a detected incompatibility between network
// snapshots from different sources (e.g. bundle vs. current environment).
type NetworkSnapshotMismatch struct {
	// Source names the origin of this snapshot (e.g. "bundle", "registry", "current").
	Source string
	// Snapshot is the network snapshot from this source.
	Snapshot *network.Snapshot
}

// NetworkSnapshotReport is the output of ValidateNetworkSnapshots.
type NetworkSnapshotReport struct {
	// OK is true when all network snapshots are compatible for replay.
	OK bool
	// Mismatches lists every detected incompatible snapshot.
	Mismatches []NetworkSnapshotMismatch
	// DiagnosticOverride is true when validation ran in diagnostic mode.
	// When true, the simulator is still started even though mismatches exist.
	DiagnosticOverride bool
	// IntentionalOverride is true when the user explicitly requested
	// cross-network analysis via the override flag.
	IntentionalOverride bool
}

// NetworkSnapshotError is returned by ValidateNetworkSnapshots when the report
// contains mismatches and diagnostic override is not enabled.
type NetworkSnapshotError struct {
	Report *NetworkSnapshotReport
}

func (e *NetworkSnapshotError) Error() string {
	return formatNetworkSnapshotReport(e.Report)
}

// IsNetworkSnapshotError reports whether err is a *NetworkSnapshotError.
func IsNetworkSnapshotError(err error) bool {
	_, ok := err.(*NetworkSnapshotError)
	return ok
}

// AsNetworkSnapshotError returns the *NetworkSnapshotError if err is one, or nil.
func AsNetworkSnapshotError(err error) *NetworkSnapshotError {
	if nse, ok := err.(*NetworkSnapshotError); ok {
		return nse
	}
	return nil
}

// ValidateNetworkSnapshotsOptions configures the behaviour of ValidateNetworkSnapshots.
type ValidateNetworkSnapshotsOptions struct {
	// DiagnosticOverride disables the hard failure so mismatches are reported
	// but do not prevent the simulator from starting.
	DiagnosticOverride bool

	// IntentionalOverride signals that the user intentionally wants to replay
	// across different network configurations. When true, the override is
	// visible in the report so it can be surfaced in diagnostic output.
	IntentionalOverride bool
}

// NetworkSnapshotOverrideName is the environment variable name that enables
// the intentional cross-network override. It is surfaced in error messages
// so users can find it.
const NetworkSnapshotOverrideName = "GLASSBOX_ALLOW_CROSS_NETWORK"

// ValidateNetworkSnapshots checks that all provided network snapshots are
// compatible for replay. Snapshots are compatible when they share the same
// network passphrase.
//
// When opts is nil, strict default behaviour applies: any mismatch causes a
// *NetworkSnapshotError to be returned and the simulator must not be started.
//
// When opts.DiagnosticOverride or opts.IntentionalOverride is true, mismatches
// are recorded but the returned error is nil, allowing the caller to proceed
// with a degraded replay. The report's DiagnosticOverride or IntentionalOverride
// field will be true so callers can log a prominent warning.
//
// If any snapshot is nil, it is skipped during comparison (backwards
// compatibility with registries/bundles created before this feature).
func ValidateNetworkSnapshots(snapshots map[string]*network.Snapshot, opts *ValidateNetworkSnapshotsOptions) (*NetworkSnapshotReport, error) {
	if opts == nil {
		opts = &ValidateNetworkSnapshotsOptions{}
	}

	report := &NetworkSnapshotReport{
		DiagnosticOverride:  opts.DiagnosticOverride,
		IntentionalOverride: opts.IntentionalOverride,
	}

	// Collect non-nil snapshots and find the canonical passphrase.
	var canonicalPassphrase string
	var canonicalSource string
	for source, snap := range snapshots {
		if snap == nil {
			continue
		}
		if canonicalPassphrase == "" {
			canonicalPassphrase = snap.Passphrase
			canonicalSource = source
		} else if snap.Passphrase != canonicalPassphrase {
			report.Mismatches = append(report.Mismatches, NetworkSnapshotMismatch{
				Source:   source,
				Snapshot: snap,
			})
		}
	}

	report.OK = len(report.Mismatches) == 0

	if !report.OK && !opts.DiagnosticOverride && !opts.IntentionalOverride {
		return report, &NetworkSnapshotError{Report: report}
	}

	return report, nil
}

// ValidateBundleNetworkSnapshot checks a bundle's network snapshot against the
// current environment's network configuration. This is the primary entry point
// for pre-replay validation.
func ValidateBundleNetworkSnapshot(
	bundleSnapshot *network.Snapshot,
	currentSnapshot *network.Snapshot,
	opts *ValidateNetworkSnapshotsOptions,
) (*NetworkSnapshotReport, error) {
	snapshots := map[string]*network.Snapshot{
		"bundle":   bundleSnapshot,
		"current":  currentSnapshot,
	}
	return ValidateNetworkSnapshots(snapshots, opts)
}

// formatNetworkSnapshotReport produces the human-readable multi-mismatch
// message included in NetworkSnapshotError.Error().
func formatNetworkSnapshotReport(r *NetworkSnapshotReport) string {
	if r == nil || r.OK {
		return "network snapshot validation passed"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"network snapshots are incompatible (%d mismatch(es)) — replay aborted:\n",
		len(r.Mismatches),
	))

	for i, m := range r.Mismatches {
		sb.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, m.Source, describeSnapshotDiff(m.Snapshot)))
	}

	sb.WriteString("\nRemediation:\n")
	sb.WriteString("  • Ensure the transaction was captured on the same network you are replaying.\n")
	sb.WriteString("  • Re-capture the bundle with 'glassbox debug <tx-hash>' on the correct network.\n")
	sb.WriteString(fmt.Sprintf("  • Set %s=1 to bypass this check for intentional cross-network analysis\n", NetworkSnapshotOverrideName))
	sb.WriteString("    (override; mismatches will be logged but not fatal).\n")

	return sb.String()
}

// describeSnapshotDiff returns a one-line description of a snapshot's identity
// suitable for error messages.
func describeSnapshotDiff(snap *network.Snapshot) string {
	if snap == nil {
		return "nil snapshot"
	}
	custom := ""
	if snap.IsCustom {
		custom = " [custom]"
	}
	return fmt.Sprintf("network=%q passphrase=%q%s",
		snap.Name, truncateStr(snap.Passphrase, 40), custom)
}

// truncateStr shortens s to maxLen characters, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
