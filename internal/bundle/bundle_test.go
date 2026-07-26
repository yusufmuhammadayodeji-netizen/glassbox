// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package bundle_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dotandev/glassbox/internal/bundle"
	"github.com/dotandev/glassbox/internal/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func validNetwork() bundle.NetworkIdentity {
	return bundle.NetworkIdentity{
		Name:       "testnet",
		Passphrase: "Test SDF Network ; September 2015",
	}
}

func validLedgerState() map[string]string {
	return map[string]string{
		"AAAA": "BBBB",
		"CCCC": "DDDD",
	}
}

func validManifest() *bundle.Manifest {
	return bundle.New(
		"0.0.0-test",
		"abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		1000,
		22,
		validNetwork(),
		"envelopeXDRdata==",
		"resultMetaXDRdata==",
		validLedgerState(),
	)
}

// ── round-trip ────────────────────────────────────────────────────────────────

func TestManifest_SaveAndLoad(t *testing.T) {
	m := validManifest()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	err := m.SaveToFile(path)
	require.NoError(t, err)

	loaded, err := bundle.LoadFromFile(path)
	require.NoError(t, err)

	assert.Equal(t, m.Provenance.TxHash, loaded.Provenance.TxHash)
	assert.Equal(t, m.Network.Name, loaded.Network.Name)
	assert.Equal(t, m.Transaction.EnvelopeXDR, loaded.Transaction.EnvelopeXDR)
	assert.Equal(t, m.Transaction.ResultMetaXDR, loaded.Transaction.ResultMetaXDR)
	assert.Equal(t, m.LedgerState, loaded.LedgerState)
	assert.Equal(t, m.Checksums, loaded.Checksums)
}

// ── verify integrity ──────────────────────────────────────────────────────────

func TestManifest_Verify_OK(t *testing.T) {
	m := validManifest()
	report := m.Verify()
	assert.True(t, report.OK)
	assert.Empty(t, report.FieldErrors)
	assert.Empty(t, report.MissingMembers)
}

func TestManifest_Verify_EnvelopeTampered(t *testing.T) {
	m := validManifest()
	m.Transaction.EnvelopeXDR = "TAMPERED"
	report := m.Verify()
	assert.False(t, report.OK)
	assert.Contains(t, report.FieldErrors, bundle.MemberEnvelopeXDR)
}

func TestManifest_Verify_ResultMetaTampered(t *testing.T) {
	m := validManifest()
	m.Transaction.ResultMetaXDR = "TAMPERED"
	report := m.Verify()
	assert.False(t, report.OK)
	assert.Contains(t, report.FieldErrors, bundle.MemberResultMetaXDR)
}

func TestManifest_Verify_LedgerStateTampered(t *testing.T) {
	m := validManifest()
	m.LedgerState["AAAA"] = "TAMPERED"
	report := m.Verify()
	assert.False(t, report.OK)
	assert.Contains(t, report.FieldErrors, bundle.MemberLedgerState)
}

func TestManifest_Verify_ProvenanceTampered(t *testing.T) {
	m := validManifest()
	m.Provenance.TxHash = "tampered"
	report := m.Verify()
	assert.False(t, report.OK)
	assert.Contains(t, report.FieldErrors, bundle.MemberProvenance)
}

func TestManifest_Verify_NetworkTampered(t *testing.T) {
	m := validManifest()
	m.Network.Name = "mainnet" // changed after checksum
	report := m.Verify()
	assert.False(t, report.OK)
	assert.Contains(t, report.FieldErrors, bundle.MemberNetwork)
}

func TestManifest_Verify_MissingChecksum(t *testing.T) {
	m := validManifest()
	delete(m.Checksums, bundle.MemberEnvelopeXDR)
	report := m.Verify()
	assert.False(t, report.OK)
	assert.Contains(t, report.MissingMembers, bundle.MemberEnvelopeXDR)
}

// ── validate ─────────────────────────────────────────────────────────────────

func TestManifest_Validate_OK(t *testing.T) {
	m := validManifest()
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_WrongFormatVersion(t *testing.T) {
	m := validManifest()
	m.FormatVersion = 99
	err := m.Validate()
	require.Error(t, err)
	assert.True(t, bundle.IsValidationError(err))
}

func TestManifest_Validate_MissingTxHash(t *testing.T) {
	m := validManifest()
	m.Provenance.TxHash = ""
	// Re-compute checksums so the format doesn't fail for the wrong reason.
	m.Checksums = nil
	// We need to rebuild so checksums match; instead build from scratch with empty hash.
	m2 := bundle.New("dev", "", 0, 22, validNetwork(), "env==", "meta==", validLedgerState())
	err := m2.Validate()
	require.Error(t, err)
	assert.True(t, bundle.IsValidationError(err))
}

func TestManifest_Validate_EmptyLedgerState(t *testing.T) {
	m := bundle.New("dev", "abc123", 1, 22, validNetwork(), "env==", "meta==", map[string]string{})
	err := m.Validate()
	require.Error(t, err)
	assert.True(t, bundle.IsValidationError(err))
}

func TestManifest_Validate_ChecksumMismatch_Error(t *testing.T) {
	m := validManifest()
	m.Transaction.EnvelopeXDR = "CHANGED_AFTER_CHECKSUM"
	err := m.Validate()
	require.Error(t, err)
	assert.True(t, bundle.IsChecksumMismatch(err))
	assert.Contains(t, err.Error(), bundle.MemberEnvelopeXDR)
}

// ── field-specific errors ─────────────────────────────────────────────────────

func TestLoadFromFile_MissingFile(t *testing.T) {
	_, err := bundle.LoadFromFile("/nonexistent/path/file.json")
	require.Error(t, err)
}

func TestLoadFromFile_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json {{{"), 0600))
	_, err := bundle.LoadFromFile(path)
	require.Error(t, err)
}

// ── no credentials ────────────────────────────────────────────────────────────

func TestManifest_ContainsCredentials_False(t *testing.T) {
	m := validManifest()
	assert.False(t, m.ContainsCredentials(), "a clean bundle must not contain credentials")
}

// ── ledger state ordering independence ────────────────────────────────────────

func TestManifest_LedgerStateChecksumOrderIndependent(t *testing.T) {
	// Build two manifests with the same entries in different map iteration orders.
	state1 := map[string]string{"key_b": "val_b", "key_a": "val_a"}
	state2 := map[string]string{"key_a": "val_a", "key_b": "val_b"}

	m1 := bundle.New("dev", "hash", 1, 22, validNetwork(), "env==", "meta==", state1)
	m2 := bundle.New("dev", "hash", 1, 22, validNetwork(), "env==", "meta==", state2)

	assert.Equal(t, m1.Checksums[bundle.MemberLedgerState], m2.Checksums[bundle.MemberLedgerState],
		"ledger state checksum must be order-independent")
}

// ── suggested filename ────────────────────────────────────────────────────────

func TestSuggestedFilename(t *testing.T) {
	name := bundle.SuggestedFilename("abcdef1234567890")
	assert.Contains(t, name, "glassbox-bundle-")
	assert.Contains(t, name, ".json")
}

func TestSuggestedFilename_ShortHash(t *testing.T) {
	name := bundle.SuggestedFilename("abc")
	assert.Contains(t, name, "abc")
}

// ── integration: create then replay produces same inputs ─────────────────────

func TestManifest_ReplayInputsPreserved(t *testing.T) {
	envelopeXDR := "SGVsbG8gV29ybGQ=" // "Hello World"
	resultMetaXDR := "Rmlyc3QgbGVkZ2Vy"
	state := map[string]string{"key": "value"}

	m := bundle.New("0.1.0", "txhash", 42, 22,
		bundle.NetworkIdentity{Name: "testnet", Passphrase: "passphrase"},
		envelopeXDR, resultMetaXDR, state,
	)

	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.json")
	require.NoError(t, m.SaveToFile(path))

	loaded, err := bundle.LoadFromFile(path)
	require.NoError(t, err)

	assert.Equal(t, envelopeXDR, loaded.Transaction.EnvelopeXDR)
	assert.Equal(t, resultMetaXDR, loaded.Transaction.ResultMetaXDR)
	assert.Equal(t, state, loaded.LedgerState)
	assert.Equal(t, "testnet", loaded.Network.Name)
	assert.Equal(t, uint32(22), loaded.Provenance.ProtocolVersion)
	assert.Equal(t, uint32(42), loaded.Provenance.LedgerSequence)
	assert.True(t, loaded.Verify().OK, "loaded bundle must pass integrity check")
}

// ── NetworkSnapshot integration ──────────────────────────────────────────────

func TestManifest_WithNetworkSnapshot_RoundTrip(t *testing.T) {
	netSnap := network.NewSnapshot(
		"testnet",
		"Test SDF Network ; September 2015",
		"https://horizon-testnet.stellar.org",
		"https://soroban-testnet.stellar.org",
		22,
	)

	m := bundle.NewWithSnapshot(
		"0.0.0-test",
		"abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		1000,
		22,
		validNetwork(),
		"envelopeXDRdata==",
		"resultMetaXDRdata==",
		validLedgerState(),
		netSnap,
	)

	dir := t.TempDir()
	path := filepath.Join(dir, "bundle_with_snapshot.json")
	err := m.SaveToFile(path)
	require.NoError(t, err)

	loaded, err := bundle.LoadFromFile(path)
	require.NoError(t, err)

	require.NotNil(t, loaded.NetworkSnapshot)
	assert.Equal(t, "testnet", loaded.NetworkSnapshot.Name)
	assert.Equal(t, "Test SDF Network ; September 2015", loaded.NetworkSnapshot.Passphrase)
	assert.False(t, loaded.NetworkSnapshot.IsCustom)
	assert.Equal(t, "https://horizon-testnet.stellar.org", loaded.NetworkSnapshot.HorizonURL)
	assert.Equal(t, "https://soroban-testnet.stellar.org", loaded.NetworkSnapshot.SorobanRPCURL)
	assert.Equal(t, uint32(22), loaded.NetworkSnapshot.ProtocolVersion)
	assert.True(t, loaded.Verify().OK, "loaded bundle must pass integrity check")
}

func TestManifest_WithCustomNetworkSnapshot_RoundTrip(t *testing.T) {
	netSnap := network.NewSnapshot(
		"my-private-net",
		"My Custom Private Network ; October 2024",
		"https://horizon.my-private-net.local",
		"https://soroban.my-private-net.local",
		22,
	)

	m := bundle.NewWithSnapshot(
		"0.0.0-test",
		"abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		1000,
		22,
		bundle.NetworkIdentity{Name: "my-private-net", Passphrase: "My Custom Private Network ; October 2024"},
		"envelopeXDRdata==",
		"resultMetaXDRdata==",
		validLedgerState(),
		netSnap,
	)

	dir := t.TempDir()
	path := filepath.Join(dir, "bundle_custom_net.json")
	err := m.SaveToFile(path)
	require.NoError(t, err)

	loaded, err := bundle.LoadFromFile(path)
	require.NoError(t, err)

	require.NotNil(t, loaded.NetworkSnapshot)
	assert.True(t, loaded.NetworkSnapshot.IsCustom)
	assert.Equal(t, "my-private-net", loaded.NetworkSnapshot.Name)
	assert.True(t, loaded.Verify().OK)
}

func TestManifest_NetworkSnapshot_TamperedDetection(t *testing.T) {
	netSnap := network.NewSnapshot(
		"testnet",
		"Test SDF Network ; September 2015",
		"https://horizon-testnet.stellar.org",
		"https://soroban-testnet.stellar.org",
		22,
	)

	m := bundle.NewWithSnapshot(
		"0.0.0-test",
		"abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		1000,
		22,
		validNetwork(),
		"envelopeXDRdata==",
		"resultMetaXDRdata==",
		validLedgerState(),
		netSnap,
	)

	// Tamper with the snapshot after checksum computation
	m.NetworkSnapshot.Name = "mainnet"
	report := m.Verify()
	assert.False(t, report.OK)
	assert.Contains(t, report.FieldErrors, bundle.MemberNetworkSnapshot)
}

func TestManifest_NetworkSnapshot_OmittedWhenNil(t *testing.T) {
	m := validManifest()
	assert.Nil(t, m.NetworkSnapshot)

	dir := t.TempDir()
	path := filepath.Join(dir, "bundle_no_snapshot.json")
	err := m.SaveToFile(path)
	require.NoError(t, err)

	loaded, err := bundle.LoadFromFile(path)
	require.NoError(t, err)
	assert.Nil(t, loaded.NetworkSnapshot)
	assert.True(t, loaded.Verify().OK)
}

func TestManifest_NetworkSnapshot_ChecksumIndependence(t *testing.T) {
	// Two manifests with different network snapshots should have different
	// network_snapshot checksums but the same other checksums.
	netSnap1 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon1.stellar.org", "https://rpc1.stellar.org", 22)
	netSnap2 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon2.stellar.org", "https://rpc2.stellar.org", 22)

	m1 := bundle.NewWithSnapshot("dev", "hash", 1, 22, validNetwork(),
		"env==", "meta==", validLedgerState(), netSnap1)
	m2 := bundle.NewWithSnapshot("dev", "hash", 1, 22, validNetwork(),
		"env==", "meta==", validLedgerState(), netSnap2)

	// Other checksums should be identical
	assert.Equal(t, m1.Checksums[bundle.MemberEnvelopeXDR], m2.Checksums[bundle.MemberEnvelopeXDR])
	assert.Equal(t, m1.Checksums[bundle.MemberResultMetaXDR], m2.Checksums[bundle.MemberResultMetaXDR])
	assert.Equal(t, m1.Checksums[bundle.MemberLedgerState], m2.Checksums[bundle.MemberLedgerState])
	assert.Equal(t, m1.Checksums[bundle.MemberProvenance], m2.Checksums[bundle.MemberProvenance])
	assert.Equal(t, m1.Checksums[bundle.MemberNetwork], m2.Checksums[bundle.MemberNetwork])

	// But network_snapshot checksums should differ
	assert.NotEqual(t, m1.Checksums[bundle.MemberNetworkSnapshot],
		m2.Checksums[bundle.MemberNetworkSnapshot],
		"different URLs should produce different network_snapshot checksums")
}
