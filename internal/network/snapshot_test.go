// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package network_test

import (
	"encoding/json"
	"testing"

	"github.com/dotandev/glassbox/internal/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func testnetSnapshot() *network.Snapshot {
	return network.NewSnapshot(
		"testnet",
		"Test SDF Network ; September 2015",
		"https://horizon-testnet.stellar.org",
		"https://soroban-testnet.stellar.org",
		22,
	)
}

func mainnetSnapshot() *network.Snapshot {
	return network.NewSnapshot(
		"mainnet",
		"Public Global Stellar Network ; September 2015",
		"https://horizon.stellar.org",
		"https://soroban-rpc.mainnet.stellar.org",
		22,
	)
}

func customSnapshot() *network.Snapshot {
	return network.NewSnapshot(
		"my-private-net",
		"My Custom Private Network ; October 2024",
		"https://horizon.my-private-net.local",
		"https://soroban.my-private-net.local",
		22,
	)
}

// ── NewSnapshot ──────────────────────────────────────────────────────────────

func TestNewSnapshot_StandardNetwork(t *testing.T) {
	s := testnetSnapshot()
	assert.Equal(t, "testnet", s.Name)
	assert.False(t, s.IsCustom, "testnet should not be marked as custom")
	assert.NotEmpty(t, s.Fingerprint)
}

func TestNewSnapshot_CustomNetwork(t *testing.T) {
	s := customSnapshot()
	assert.Equal(t, "my-private-net", s.Name)
	assert.True(t, s.IsCustom, "non-standard passphrase should be marked as custom")
	assert.NotEmpty(t, s.Fingerprint)
}

func TestNewSnapshot_MainnetIsNotCustom(t *testing.T) {
	s := mainnetSnapshot()
	assert.False(t, s.IsCustom)
}

func TestNewSnapshot_FuturenetIsNotCustom(t *testing.T) {
	s := network.NewSnapshot(
		"futurenet",
		"Test SDF Future Network ; October 2022",
		"https://horizon-futurenet.stellar.org",
		"https://soroban-futurenet.stellar.org",
		22,
	)
	assert.False(t, s.IsCustom)
}

// ── fingerprint determinism ──────────────────────────────────────────────────

func TestComputeFingerprint_Deterministic(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := testnetSnapshot()
	assert.Equal(t, s1.Fingerprint, s2.Fingerprint,
		"two identical snapshots must produce the same fingerprint")
}

func TestComputeFingerprint_DifferentNetworks(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := mainnetSnapshot()
	assert.NotEqual(t, s1.Fingerprint, s2.Fingerprint,
		"different networks must produce different fingerprints")
}

func TestComputeFingerprint_NilSnapshot(t *testing.T) {
	assert.Empty(t, network.ComputeFingerprint(nil))
}

func TestComputeFingerprint_URLsAffectFingerprint(t *testing.T) {
	s1 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon1.stellar.org", "https://rpc1.stellar.org", 22)
	s2 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon2.stellar.org", "https://rpc2.stellar.org", 22)
	assert.NotEqual(t, s1.Fingerprint, s2.Fingerprint,
		"different URLs should produce different fingerprints")
}

func TestComputeFingerprint_ProtocolVersionAffectsFingerprint(t *testing.T) {
	s1 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon.stellar.org", "https://rpc.stellar.org", 22)
	s2 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon.stellar.org", "https://rpc.stellar.org", 20)
	assert.NotEqual(t, s1.Fingerprint, s2.Fingerprint,
		"different protocol versions should produce different fingerprints")
}

// ── Match ────────────────────────────────────────────────────────────────────

func TestMatch_SameNetwork(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := testnetSnapshot()
	assert.True(t, s1.Match(s2))
}

func TestMatch_DifferentNetworks(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := mainnetSnapshot()
	assert.False(t, s1.Match(s2))
}

func TestMatch_StandardVsCustomWithSamePassphrase(t *testing.T) {
	// If someone manually sets a standard passphrase on a custom-named snapshot,
	// it should still match by passphrase.
	s1 := testnetSnapshot()
	s2 := network.NewSnapshot("custom-name", "Test SDF Network ; September 2015",
		"", "", 0)
	assert.True(t, s1.Match(s2), "match should be passphrase-based only")
}

func TestMatch_NilSnapshot(t *testing.T) {
	s := testnetSnapshot()
	assert.False(t, s.Match(nil))
	assert.False(t, (*network.Snapshot)(nil).Match(s))
	assert.True(t, (*network.Snapshot)(nil).Match(nil))
}

// ── MatchStrict ──────────────────────────────────────────────────────────────

func TestMatchStrict_Identical(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := testnetSnapshot()
	assert.True(t, s1.MatchStrict(s2))
}

func TestMatchStrict_DifferentURLs(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://different.horizon.org", "https://different.rpc.org", 22)
	assert.True(t, s1.Match(s2), "loose match should pass")
	assert.False(t, s1.MatchStrict(s2), "strict match should fail due to different URLs")
}

func TestMatchStrict_DifferentProtocolVersion(t *testing.T) {
	s1 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon.stellar.org", "https://rpc.stellar.org", 22)
	s2 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon.stellar.org", "https://rpc.stellar.org", 20)
	assert.False(t, s1.MatchStrict(s2))
}

// ── Compare ──────────────────────────────────────────────────────────────────

func TestCompare_Identical(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := testnetSnapshot()
	assert.Empty(t, s1.Compare(s2))
}

func TestCompare_DifferentName(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := network.NewSnapshot("testnet-2", "Test SDF Network ; September 2015",
		"https://horizon.stellar.org", "https://rpc.stellar.org", 22)
	diff := s1.Compare(s2)
	assert.Contains(t, diff, "name")
}

func TestCompare_DifferentPassphrase(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := network.NewSnapshot("testnet", "Different Passphrase",
		"https://horizon.stellar.org", "https://rpc.stellar.org", 22)
	diff := s1.Compare(s2)
	assert.Contains(t, diff, "passphrase")
}

func TestCompare_DifferentProtocolVersion(t *testing.T) {
	s1 := testnetSnapshot()
	s2 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon.stellar.org", "https://rpc.stellar.org", 20)
	diff := s1.Compare(s2)
	assert.Contains(t, diff, "protocol_version")
}

func TestCompare_NilSnapshots(t *testing.T) {
	assert.Empty(t, (*network.Snapshot)(nil).Compare(nil))
}

func TestCompare_OneNil(t *testing.T) {
	s := testnetSnapshot()
	assert.NotEmpty(t, s.Compare(nil))
	assert.NotEmpty(t, (*network.Snapshot)(nil).Compare(s))
}

// ── WellKnownPassphrases ────────────────────────────────────────────────────

func TestWellKnownPassphrases_ContainsAllStandard(t *testing.T) {
	require.Contains(t, network.WellKnownPassphrases, "testnet")
	require.Contains(t, network.WellKnownPassphrases, "mainnet")
	require.Contains(t, network.WellKnownPassphrases, "futurenet")

	assert.Equal(t, "Test SDF Network ; September 2015",
		network.WellKnownPassphrases["testnet"])
	assert.Equal(t, "Public Global Stellar Network ; September 2015",
		network.WellKnownPassphrases["mainnet"])
	assert.Equal(t, "Test SDF Future Network ; October 2022",
		network.WellKnownPassphrases["futurenet"])
}

// ── custom network passphrase mismatch ──────────────────────────────────────

func TestCustomNetwork_DoesNotMatchStandard(t *testing.T) {
	custom := customSnapshot()
	testnet := testnetSnapshot()
	mainnet := mainnetSnapshot()

	assert.False(t, custom.Match(testnet),
		"custom network must not match testnet")
	assert.False(t, custom.Match(mainnet),
		"custom network must not match mainnet")
}

func TestCustomNetwork_MatchesItself(t *testing.T) {
	c1 := customSnapshot()
	c2 := customSnapshot()
	assert.True(t, c1.Match(c2))
	assert.True(t, c1.IsCustom)
	assert.True(t, c2.IsCustom)
}

// ── JSON round-trip ─────────────────────────────────────────────────────────

func TestSnapshot_JSONRoundTrip(t *testing.T) {
	original := customSnapshot()

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded network.Snapshot
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Passphrase, decoded.Passphrase)
	assert.Equal(t, original.HorizonURL, decoded.HorizonURL)
	assert.Equal(t, original.SorobanRPCURL, decoded.SorobanRPCURL)
	assert.Equal(t, original.ProtocolVersion, decoded.ProtocolVersion)
	assert.Equal(t, original.IsCustom, decoded.IsCustom)
}

func TestSnapshot_FingerprintExcludedFromJSON(t *testing.T) {
	s := testnetSnapshot()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	// Fingerprint is included in JSON but should not affect matching
	assert.Contains(t, string(data), `"fingerprint"`)
}
