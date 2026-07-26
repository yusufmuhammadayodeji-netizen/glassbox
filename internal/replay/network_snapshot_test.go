// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"testing"

	"github.com/dotandev/glassbox/internal/network"
	"github.com/dotandev/glassbox/internal/replay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func testnetNetSnapshot() *network.Snapshot {
	return network.NewSnapshot(
		"testnet",
		"Test SDF Network ; September 2015",
		"https://horizon-testnet.stellar.org",
		"https://soroban-testnet.stellar.org",
		22,
	)
}

func mainnetNetSnapshot() *network.Snapshot {
	return network.NewSnapshot(
		"mainnet",
		"Public Global Stellar Network ; September 2015",
		"https://horizon.stellar.org",
		"https://soroban-rpc.mainnet.stellar.org",
		22,
	)
}

func customNetSnapshot() *network.Snapshot {
	return network.NewSnapshot(
		"my-private-net",
		"My Custom Private Network ; October 2024",
		"https://horizon.my-private-net.local",
		"https://soroban.my-private-net.local",
		22,
	)
}

// ── ValidateNetworkSnapshots ─────────────────────────────────────────────────

func TestValidateNetworkSnapshots_AllMatch(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":   testnetNetSnapshot(),
		"registry": testnetNetSnapshot(),
		"current":  testnetNetSnapshot(),
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
	assert.Empty(t, report.Mismatches)
}

func TestValidateNetworkSnapshots_SingleSnapshot(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"current": testnetNetSnapshot(),
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
}

func TestValidateNetworkSnapshots_NilSkipped(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  nil,
		"current": testnetNetSnapshot(),
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
}

func TestValidateNetworkSnapshots_AllNil(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  nil,
		"current": nil,
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
}

// ── passphrase mismatch ──────────────────────────────────────────────────────

func TestValidateNetworkSnapshots_PassphraseMismatch(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":   testnetNetSnapshot(),
		"current":  mainnetNetSnapshot(),
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.Error(t, err)
	assert.False(t, report.OK)
	assert.Len(t, report.Mismatches, 1)
	assert.False(t, report.DiagnosticOverride)
	assert.False(t, report.IntentionalOverride)

	nse := replay.AsNetworkSnapshotError(err)
	require.NotNil(t, nse)
	assert.Contains(t, err.Error(), "network snapshots are incompatible")
}

func TestValidateNetworkSnapshots_ThreeWayMismatch(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":   testnetNetSnapshot(),
		"registry": mainnetNetSnapshot(),
		"current":  customNetSnapshot(),
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.Error(t, err)
	assert.False(t, report.OK)
	// Two mismatches: mainnet vs testnet, custom vs testnet
	assert.GreaterOrEqual(t, len(report.Mismatches), 2)
}

// ── custom network mismatches ────────────────────────────────────────────────

func TestValidateNetworkSnapshots_CustomVsTestnet(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  customNetSnapshot(),
		"current": testnetNetSnapshot(),
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.Error(t, err)
	assert.False(t, report.OK)
	assert.Len(t, report.Mismatches, 1)
}

func TestValidateNetworkSnapshots_TwoCustomSamePassphrase(t *testing.T) {
	// Two custom snapshots with the same passphrase should match
	s1 := network.NewSnapshot("custom-a", "Same Passphrase ; 2024", "", "", 22)
	s2 := network.NewSnapshot("custom-b", "Same Passphrase ; 2024", "", "", 22)
	snapshots := map[string]*network.Snapshot{
		"source1": s1,
		"source2": s2,
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
}

// ── diagnostic override ──────────────────────────────────────────────────────

func TestValidateNetworkSnapshots_DiagnosticOverride(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  testnetNetSnapshot(),
		"current": mainnetNetSnapshot(),
	}
	opts := &replay.ValidateNetworkSnapshotsOptions{
		DiagnosticOverride: true,
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, opts)
	require.NoError(t, err, "diagnostic override must suppress the error")
	assert.False(t, report.OK, "OK must still be false when mismatches exist")
	assert.True(t, report.DiagnosticOverride)
	assert.NotEmpty(t, report.Mismatches)
}

// ── intentional override ─────────────────────────────────────────────────────

func TestValidateNetworkSnapshots_IntentionalOverride(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  testnetNetSnapshot(),
		"current": mainnetNetSnapshot(),
	}
	opts := &replay.ValidateNetworkSnapshotsOptions{
		IntentionalOverride: true,
	}
	report, err := replay.ValidateNetworkSnapshots(snapshots, opts)
	require.NoError(t, err, "intentional override must suppress the error")
	assert.False(t, report.OK)
	assert.True(t, report.IntentionalOverride)
	assert.NotEmpty(t, report.Mismatches)
}

// ── ValidateBundleNetworkSnapshot ────────────────────────────────────────────

func TestValidateBundleNetworkSnapshot_Match(t *testing.T) {
	bundle := testnetNetSnapshot()
	current := testnetNetSnapshot()
	report, err := replay.ValidateBundleNetworkSnapshot(bundle, current, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
}

func TestValidateBundleNetworkSnapshot_Mismatch(t *testing.T) {
	bundle := testnetNetSnapshot()
	current := mainnetNetSnapshot()
	report, err := replay.ValidateBundleNetworkSnapshot(bundle, current, nil)
	require.Error(t, err)
	assert.False(t, report.OK)
}

func TestValidateBundleNetworkSnapshot_NilBundleSnapshot(t *testing.T) {
	current := testnetNetSnapshot()
	report, err := replay.ValidateBundleNetworkSnapshot(nil, current, nil)
	require.NoError(t, err)
	assert.True(t, report.OK, "nil bundle snapshot should be skipped")
}

func TestValidateBundleNetworkSnapshot_NilCurrentSnapshot(t *testing.T) {
	bundle := testnetNetSnapshot()
	report, err := replay.ValidateBundleNetworkSnapshot(bundle, nil, nil)
	require.NoError(t, err)
	assert.True(t, report.OK, "nil current snapshot should be skipped")
}

func TestValidateBundleNetworkSnapshot_BothNil(t *testing.T) {
	report, err := replay.ValidateBundleNetworkSnapshot(nil, nil, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
}

// ── error identification helpers ─────────────────────────────────────────────

func TestIsNetworkSnapshotError(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  testnetNetSnapshot(),
		"current": mainnetNetSnapshot(),
	}
	_, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.Error(t, err)
	assert.True(t, replay.IsNetworkSnapshotError(err))
	assert.NotNil(t, replay.AsNetworkSnapshotError(err))
}

func TestNetworkSnapshotError_ContainsOverrideHint(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  testnetNetSnapshot(),
		"current": mainnetNetSnapshot(),
	}
	_, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), replay.NetworkSnapshotOverrideName)
}

func TestNetworkSnapshotError_ContainsRemediation(t *testing.T) {
	snapshots := map[string]*network.Snapshot{
		"bundle":  testnetNetSnapshot(),
		"current": mainnetNetSnapshot(),
	}
	_, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Remediation")
}

// ── empty snapshots map ─────────────────────────────────────────────────────

func TestValidateNetworkSnapshots_EmptyMap(t *testing.T) {
	snapshots := map[string]*network.Snapshot{}
	report, err := replay.ValidateNetworkSnapshots(snapshots, nil)
	require.NoError(t, err)
	assert.True(t, report.OK)
}
