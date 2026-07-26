// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dotandev/glassbox/internal/network"
	"github.com/dotandev/glassbox/internal/snapshot"
)

// helpers

func makeSnap(t *testing.T, entries map[string]string) *snapshot.Snapshot {
	t.Helper()
	return snapshot.FromMap(entries)
}

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	return New("v1.0.0", "abc123", "testnet", "envelopeXDR", "resultMetaXDR")
}

// --- New ---

func TestNew_Fields(t *testing.T) {
	r := New("v2.0.0", "txhash", "mainnet", "env", "meta")

	if r.SchemaVersion != SchemaVersion {
		t.Errorf("expected schema version %d, got %d", SchemaVersion, r.SchemaVersion)
	}
	if r.GlassboxVersion != "v2.0.0" {
		t.Errorf("expected version v2.0.0, got %s", r.GlassboxVersion)
	}
	if r.TxHash != "txhash" {
		t.Errorf("expected txhash, got %s", r.TxHash)
	}
	if r.Network != "mainnet" {
		t.Errorf("expected mainnet, got %s", r.Network)
	}
	if r.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if len(r.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(r.Entries))
	}
}

// --- Add ---

func TestAdd_AppendsEntry(t *testing.T) {
	r := newTestRegistry(t)
	snap := makeSnap(t, map[string]string{"key-a": "val-a"})

	r.Add(1000, snap)

	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
	if r.Entries[0].Timestamp != 1000 {
		t.Errorf("expected timestamp 1000, got %d", r.Entries[0].Timestamp)
	}
	if r.Entries[0].Checksum == "" {
		t.Error("expected non-empty checksum")
	}
}

func TestAdd_MultipleEntries(t *testing.T) {
	r := newTestRegistry(t)
	for i := int64(0); i < 5; i++ {
		r.Add(i*100, makeSnap(t, map[string]string{"k": "v"}))
	}
	if len(r.Entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(r.Entries))
	}
}

func TestAdd_ChecksumIsDeterministic(t *testing.T) {
	snap := makeSnap(t, map[string]string{"key": "value"})

	r1 := newTestRegistry(t)
	r1.Add(0, snap)

	r2 := newTestRegistry(t)
	r2.Add(0, snap)

	if r1.Entries[0].Checksum != r2.Entries[0].Checksum {
		t.Errorf("checksums differ: %s vs %s", r1.Entries[0].Checksum, r2.Entries[0].Checksum)
	}
}

func TestAdd_NilSnapshot(t *testing.T) {
	r := newTestRegistry(t)
	r.Add(0, nil)
	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
	if r.Entries[0].Checksum != "" {
		t.Errorf("expected empty checksum for nil snapshot, got %s", r.Entries[0].Checksum)
	}
}

// --- SaveToFile / LoadFromFile ---

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	r := newTestRegistry(t)
	r.Add(1000, makeSnap(t, map[string]string{"key-1": "val-1"}))
	r.Add(2000, makeSnap(t, map[string]string{"key-2": "val-2"}))

	path := filepath.Join(t.TempDir(), "registry.json")
	if err := r.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if loaded.TxHash != r.TxHash {
		t.Errorf("expected TxHash %s, got %s", r.TxHash, loaded.TxHash)
	}
	if loaded.Network != r.Network {
		t.Errorf("expected Network %s, got %s", r.Network, loaded.Network)
	}
	if len(loaded.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Timestamp != 1000 {
		t.Errorf("expected timestamp 1000, got %d", loaded.Entries[0].Timestamp)
	}
}

func TestSaveToFile_Atomic(t *testing.T) {
	r := newTestRegistry(t)
	r.Add(0, makeSnap(t, map[string]string{"k": "v"}))

	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	if err := r.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("expected temp file to be cleaned up")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected registry file to exist: %v", err)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/registry.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFromFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadFromFile_MissingTxHash_ReturnsError(t *testing.T) {
	r := newTestRegistry(t)
	r.TxHash = "" // empty tx hash

	data, _ := json.MarshalIndent(r, "", "  ")
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected error for empty TxHash")
	}
	if !strings.Contains(err.Error(), "transaction hash") {
		t.Errorf("error should mention 'transaction hash', got: %v", err)
	}
}

func TestLoadFromFile_MissingNetwork_ReturnsError(t *testing.T) {
	r := newTestRegistry(t)
	r.Network = "" // empty network

	data, _ := json.MarshalIndent(r, "", "  ")
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected error for empty Network")
	}
	if !strings.Contains(err.Error(), "network") {
		t.Errorf("error should mention 'network', got: %v", err)
	}
}

func TestLoadFromFile_MissingGlassboxVersion_Backfilled(t *testing.T) {
	r := newTestRegistry(t)
	r.GlassboxVersion = ""

	data, _ := json.MarshalIndent(r, "", "  ")
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.GlassboxVersion != "unknown" {
		t.Errorf("expected GlassboxVersion to be back-filled to 'unknown', got %q", loaded.GlassboxVersion)
	}
}

func TestLoadFromFile_WrongSchemaVersion(t *testing.T) {
	r := newTestRegistry(t)
	r.SchemaVersion = 999

	data, _ := json.MarshalIndent(r, "", "  ")
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Error("expected error for wrong schema version")
	}
	if !strings.Contains(err.Error(), "schema version") {
		t.Errorf("expected schema version error, got: %v", err)
	}
}

func TestSaveToFile_PreservesCreatedAt(t *testing.T) {
	r := newTestRegistry(t)
	original := r.CreatedAt

	path := filepath.Join(t.TempDir(), "registry.json")
	if err := r.SaveToFile(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	diff := loaded.CreatedAt.Sub(original)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("CreatedAt changed after round-trip: %v vs %v", original, loaded.CreatedAt)
	}
}

// --- VerifyIntegrity ---

func TestVerifyIntegrity_Clean(t *testing.T) {
	r := newTestRegistry(t)
	r.Add(0, makeSnap(t, map[string]string{"k": "v"}))
	r.Add(1, makeSnap(t, map[string]string{"k": "v2"}))

	errs := r.VerifyIntegrity()
	if len(errs) != 0 {
		t.Errorf("expected no integrity errors, got: %v", errs)
	}
}

func TestVerifyIntegrity_TamperedChecksum(t *testing.T) {
	r := newTestRegistry(t)
	r.Add(0, makeSnap(t, map[string]string{"k": "v"}))
	r.Entries[0].Checksum = "deadbeefdeadbeef"

	errs := r.VerifyIntegrity()
	if len(errs) != 1 {
		t.Errorf("expected 1 integrity error, got %d", len(errs))
	}
}

func TestVerifyIntegrity_MissingChecksumBackfilled(t *testing.T) {
	r := newTestRegistry(t)
	snap := makeSnap(t, map[string]string{"k": "v"})
	r.Entries = append(r.Entries, Entry{
		Timestamp: 0,
		Snapshot:  snap,
		Checksum:  "",
	})

	errs := r.VerifyIntegrity()
	if len(errs) != 0 {
		t.Errorf("expected no errors for missing checksum (back-fill), got: %v", errs)
	}
	if r.Entries[0].Checksum == "" {
		t.Error("expected checksum to be back-filled")
	}
}

func TestVerifyIntegrity_MultipleErrors(t *testing.T) {
	r := newTestRegistry(t)
	r.Add(0, makeSnap(t, map[string]string{"k": "v"}))
	r.Add(1, makeSnap(t, map[string]string{"k": "v2"}))
	r.Entries[0].Checksum = "bad1"
	r.Entries[1].Checksum = "bad2"

	errs := r.VerifyIntegrity()
	if len(errs) != 2 {
		t.Errorf("expected 2 integrity errors, got %d", len(errs))
	}
}

// --- CommandFingerprint ---

func TestCommandFingerprint_SetAndMatch(t *testing.T) {
	r := newTestRegistry(t)
	params := map[string]string{"network": "testnet", "tx": "abc123"}
	r.SetCommandFingerprint(params)

	if r.CommandFingerprint == "" {
		t.Error("expected non-empty command fingerprint")
	}
	if !r.MatchesCommandFingerprint(params) {
		t.Error("expected fingerprint to match same params")
	}
}

func TestCommandFingerprint_Mismatch(t *testing.T) {
	r := newTestRegistry(t)
	r.SetCommandFingerprint(map[string]string{"network": "testnet"})

	if r.MatchesCommandFingerprint(map[string]string{"network": "mainnet"}) {
		t.Error("expected fingerprint mismatch for different params")
	}
}

func TestCommandFingerprint_EmptyAlwaysMatches(t *testing.T) {
	r := newTestRegistry(t)
	if !r.MatchesCommandFingerprint(map[string]string{"any": "params"}) {
		t.Error("expected empty fingerprint to always match")
	}
}

func TestCommandFingerprint_Deterministic(t *testing.T) {
	params := map[string]string{"a": "1", "b": "2", "c": "3"}
	r1 := newTestRegistry(t)
	r1.SetCommandFingerprint(params)
	r2 := newTestRegistry(t)
	r2.SetCommandFingerprint(params)

	if r1.CommandFingerprint != r2.CommandFingerprint {
		t.Errorf("fingerprints differ: %s vs %s", r1.CommandFingerprint, r2.CommandFingerprint)
	}
}

// --- Full round-trip with integrity check ---

func TestFullRoundTrip_WithIntegrityCheck(t *testing.T) {
	r := newTestRegistry(t)
	r.Add(1000, makeSnap(t, map[string]string{"ledger-key-1": "ledger-val-1"}))
	r.Add(2000, makeSnap(t, map[string]string{"ledger-key-2": "ledger-val-2"}))
	r.SetCommandFingerprint(map[string]string{"network": "testnet", "tx": "abc123"})

	path := filepath.Join(t.TempDir(), "full.json")
	if err := r.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	errs := loaded.VerifyIntegrity()
	if len(errs) != 0 {
		t.Errorf("integrity check failed after round-trip: %v", errs)
	}

	if !loaded.MatchesCommandFingerprint(map[string]string{"network": "testnet", "tx": "abc123"}) {
		t.Error("command fingerprint mismatch after round-trip")
	}

	m := loaded.Entries[0].Snapshot.ToMap()
	if m["ledger-key-1"] != "ledger-val-1" {
		t.Errorf("snapshot data corrupted: %v", m)
	}
}

// --- computeSnapshotChecksum ---

func TestComputeSnapshotChecksum_Nil(t *testing.T) {
	if got := computeSnapshotChecksum(nil); got != "" {
		t.Errorf("expected empty checksum for nil, got %s", got)
	}
}

func TestComputeSnapshotChecksum_Deterministic(t *testing.T) {
	snap := makeSnap(t, map[string]string{"k": "v"})
	c1 := computeSnapshotChecksum(snap)
	c2 := computeSnapshotChecksum(snap)
	if c1 != c2 {
		t.Errorf("checksums differ: %s vs %s", c1, c2)
	}
}

func TestComputeSnapshotChecksum_ChangesOnMutation(t *testing.T) {
	s1 := makeSnap(t, map[string]string{"k": "v1"})
	s2 := makeSnap(t, map[string]string{"k": "v2"})
	if computeSnapshotChecksum(s1) == computeSnapshotChecksum(s2) {
		t.Error("expected different checksums for different snapshots")
	}
}

// --- NetworkSnapshot integration ---

func TestNewWithSnapshot_Fields(t *testing.T) {
	netSnap := network.NewSnapshot(
		"testnet",
		"Test SDF Network ; September 2015",
		"https://horizon-testnet.stellar.org",
		"https://soroban-testnet.stellar.org",
		22,
	)

	r := NewWithSnapshot("v1.0.0", "abc123", "testnet", "envelopeXDR", "resultMetaXDR", netSnap)

	if r.NetworkSnapshot == nil {
		t.Fatal("expected non-nil NetworkSnapshot")
	}
	if r.NetworkSnapshot.Name != "testnet" {
		t.Errorf("expected network name testnet, got %s", r.NetworkSnapshot.Name)
	}
	if r.NetworkSnapshot.Passphrase != "Test SDF Network ; September 2015" {
		t.Errorf("unexpected passphrase: %s", r.NetworkSnapshot.Passphrase)
	}
	if r.NetworkSnapshot.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}
}

func TestNewWithSnapshot_NilSnapshot(t *testing.T) {
	r := NewWithSnapshot("v1.0.0", "abc123", "testnet", "envelopeXDR", "resultMetaXDR", nil)
	if r.NetworkSnapshot != nil {
		t.Error("expected nil NetworkSnapshot when nil is passed")
	}
}

func TestNew_BackwardCompatible(t *testing.T) {
	r := New("v1.0.0", "abc123", "testnet", "envelopeXDR", "resultMetaXDR")
	if r.NetworkSnapshot != nil {
		t.Error("expected nil NetworkSnapshot from legacy New()")
	}
}

func TestRegistry_WithNetworkSnapshot_RoundTrip(t *testing.T) {
	netSnap := network.NewSnapshot(
		"testnet",
		"Test SDF Network ; September 2015",
		"https://horizon-testnet.stellar.org",
		"https://soroban-testnet.stellar.org",
		22,
	)

	r := NewWithSnapshot("v1.0.0", "abc123", "testnet", "envelopeXDR", "resultMetaXDR", netSnap)
	r.Add(1000, makeSnap(t, map[string]string{"k": "v"}))

	path := filepath.Join(t.TempDir(), "registry_snapshot.json")
	if err := r.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if loaded.NetworkSnapshot == nil {
		t.Fatal("expected non-nil NetworkSnapshot after round-trip")
	}
	if loaded.NetworkSnapshot.Name != "testnet" {
		t.Errorf("expected name testnet, got %s", loaded.NetworkSnapshot.Name)
	}
	if loaded.NetworkSnapshot.Passphrase != "Test SDF Network ; September 2015" {
		t.Errorf("unexpected passphrase: %s", loaded.NetworkSnapshot.Passphrase)
	}
}

func TestRegistry_WithCustomNetworkSnapshot_RoundTrip(t *testing.T) {
	netSnap := network.NewSnapshot(
		"my-private-net",
		"My Custom Private Network ; October 2024",
		"https://horizon.my-private-net.local",
		"https://soroban.my-private-net.local",
		22,
	)

	r := NewWithSnapshot("v1.0.0", "abc123", "my-private-net", "envelopeXDR", "resultMetaXDR", netSnap)

	path := filepath.Join(t.TempDir(), "registry_custom.json")
	if err := r.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if loaded.NetworkSnapshot == nil {
		t.Fatal("expected non-nil NetworkSnapshot after round-trip")
	}
	if !loaded.NetworkSnapshot.IsCustom {
		t.Error("expected IsCustom to be true")
	}
	if loaded.NetworkSnapshot.Name != "my-private-net" {
		t.Errorf("expected name my-private-net, got %s", loaded.NetworkSnapshot.Name)
	}
}

func TestRegistry_NetworkSnapshot_OmittedWhenNil(t *testing.T) {
	r := New("v1.0.0", "abc123", "testnet", "envelopeXDR", "resultMetaXDR")

	path := filepath.Join(t.TempDir(), "registry_no_snapshot.json")
	if err := r.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	// Verify the JSON does not contain network_snapshot
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "network_snapshot") {
		t.Error("expected network_snapshot to be omitted when nil")
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if loaded.NetworkSnapshot != nil {
		t.Error("expected nil NetworkSnapshot after round-trip")
	}
}

func TestRegistry_NetworkSnapshot_PassphraseMismatch(t *testing.T) {
	snap1 := network.NewSnapshot("testnet", "Test SDF Network ; September 2015",
		"https://horizon-testnet.stellar.org", "https://soroban-testnet.stellar.org", 22)
	snap2 := network.NewSnapshot("mainnet", "Public Global Stellar Network ; September 2015",
		"https://horizon.stellar.org", "https://soroban-rpc.mainnet.stellar.org", 22)

	if snap1.Match(snap2) {
		t.Error("expected testnet and mainnet snapshots to not match")
	}
	if !snap1.Match(network.NewSnapshot("testnet", "Test SDF Network ; September 2015", "", "", 0)) {
		t.Error("expected same passphrase to match regardless of URLs")
	}
}

func TestRegistry_NetworkSnapshot_JSONOmitEmpty(t *testing.T) {
	r := New("v1.0.0", "abc123", "testnet", "envelopeXDR", "resultMetaXDR")

	data, _ := json.Marshal(r)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, exists := raw["network_snapshot"]; exists {
		t.Error("network_snapshot should be omitted from JSON when nil")
	}
}
