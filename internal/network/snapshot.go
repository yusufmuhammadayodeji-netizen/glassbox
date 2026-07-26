// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Package network provides deterministic network configuration snapshots for
// capturing and comparing the full network context of a Stellar transaction.
//
// A NetworkSnapshot captures the network name, passphrase, RPC identity, and
// protocol metadata into a canonical type that can be serialized into bundle
// manifests and replay registries. Snapshots use deterministic serialization
// so identical configurations always produce the same fingerprint, enabling
// reliable comparison before replay.
package network

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// WellKnownPassphrases maps standard Stellar network names to their canonical
// passphrases. Custom networks will not appear in this map.
var WellKnownPassphrases = map[string]string{
	"testnet":   "Test SDF Network ; September 2015",
	"mainnet":   "Public Global Stellar Network ; September 2015",
	"futurenet": "Test SDF Future Network ; October 2022",
}

// Snapshot captures the full network configuration at a point in time.
// It is designed for deterministic serialization so that two snapshots with
// identical configurations always produce the same JSON fingerprint.
type Snapshot struct {
	// Name is the canonical network name (testnet, mainnet, futurenet, or a
	// custom name for private networks).
	Name string `json:"name"`

	// Passphrase is the Stellar network passphrase required for transaction
	// signing. For well-known networks this matches the canonical passphrase;
	// for custom networks it is the user-supplied value.
	Passphrase string `json:"passphrase"`

	// HorizonURL is the Horizon server URL used for this network. Empty when
	// not available (e.g. offline replay).
	HorizonURL string `json:"horizon_url,omitempty"`

	// SorobanRPCURL is the Soroban RPC endpoint URL. Empty when not available.
	SorobanRPCURL string `json:"soroban_rpc_url,omitempty"`

	// ProtocolVersion is the Soroban protocol version at capture time.
	ProtocolVersion uint32 `json:"protocol_version,omitempty"`

	// IsCustom reports whether this snapshot represents a custom (non-standard)
	// network configuration. Custom networks are identified by having a
	// passphrase that does not match any well-known network.
	IsCustom bool `json:"is_custom"`

	// Fingerprint is the SHA-256 hex digest of the deterministic serialization
	// of this snapshot. It is computed on demand via ComputeFingerprint and is
	// not included in the hash input itself.
	Fingerprint string `json:"fingerprint,omitempty"`
}

// NewSnapshot creates a NetworkSnapshot from the given parameters. The IsCustom
// flag is determined automatically by comparing the passphrase against the
// well-known network passphrases.
func NewSnapshot(name, passphrase, horizonURL, sorobanRPCURL string, protocolVersion uint32) *Snapshot {
	s := &Snapshot{
		Name:            name,
		Passphrase:      passphrase,
		HorizonURL:      horizonURL,
		SorobanRPCURL:   sorobanRPCURL,
		ProtocolVersion: protocolVersion,
		IsCustom:        !isWellKnownPassphrase(passphrase),
	}
	s.Fingerprint = ComputeFingerprint(s)
	return s
}

// ComputeFingerprint returns a deterministic SHA-256 hex digest of the snapshot.
// The hash is computed over a canonical JSON encoding with sorted keys so that
// field ordering in the struct definition does not affect the result.
func ComputeFingerprint(s *Snapshot) string {
	if s == nil {
		return ""
	}
	data := canonicalBytes(s)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Match reports whether two snapshots are compatible for replay. Two snapshots
// match when they have the same network passphrase. The name, URLs, and
// protocol version are not compared because the passphrase is the authoritative
// identity of a Stellar network.
func (s *Snapshot) Match(other *Snapshot) bool {
	if s == nil || other == nil {
		return s == other
	}
	return s.Passphrase == other.Passphrase
}

// MatchStrict reports whether two snapshots are strictly identical. All fields
// except Fingerprint must match. This is useful for detecting environment drift
// (e.g. different RPC endpoints for the same network).
func (s *Snapshot) MatchStrict(other *Snapshot) bool {
	if s == nil || other == nil {
		return s == other
	}
	return s.Name == other.Name &&
		s.Passphrase == other.Passphrase &&
		s.HorizonURL == other.HorizonURL &&
		s.SorobanRPCURL == other.SorobanRPCURL &&
		s.ProtocolVersion == other.ProtocolVersion &&
		s.IsCustom == other.IsCustom
}

// Compare returns a human-readable description of the differences between two
// snapshots. An empty string indicates the snapshots match.
func (s *Snapshot) Compare(other *Snapshot) string {
	if s == nil && other == nil {
		return ""
	}
	if s == nil {
		return "current snapshot is nil but other is not"
	}
	if other == nil {
		return "other snapshot is nil but current is not"
	}

	var diffs []string
	if s.Name != other.Name {
		diffs = append(diffs, fmt.Sprintf("name: %q vs %q", s.Name, other.Name))
	}
	if s.Passphrase != other.Passphrase {
		diffs = append(diffs, fmt.Sprintf("passphrase: %q vs %q", truncate(s.Passphrase, 40), truncate(other.Passphrase, 40)))
	}
	if s.HorizonURL != other.HorizonURL {
		diffs = append(diffs, fmt.Sprintf("horizon_url: %q vs %q", s.HorizonURL, other.HorizonURL))
	}
	if s.SorobanRPCURL != other.SorobanRPCURL {
		diffs = append(diffs, fmt.Sprintf("soroban_rpc_url: %q vs %q", s.SorobanRPCURL, other.SorobanRPCURL))
	}
	if s.ProtocolVersion != other.ProtocolVersion {
		diffs = append(diffs, fmt.Sprintf("protocol_version: %d vs %d", s.ProtocolVersion, other.ProtocolVersion))
	}
	if s.IsCustom != other.IsCustom {
		diffs = append(diffs, fmt.Sprintf("is_custom: %v vs %v", s.IsCustom, other.IsCustom))
	}

	if len(diffs) == 0 {
		return ""
	}
	return "network snapshot mismatch: " + strings.Join(diffs, "; ")
}

// isWellKnownPassphrase returns true if the passphrase matches a standard
// Stellar network passphrase.
func isWellKnownPassphrase(passphrase string) bool {
	for _, known := range WellKnownPassphrases {
		if passphrase == known {
			return true
		}
	}
	return false
}

// canonicalBytes produces a deterministic JSON encoding of the snapshot with
// sorted map keys. This is used for fingerprint computation.
func canonicalBytes(s *Snapshot) []byte {
	// Build a sorted map to ensure deterministic key ordering.
	m := make(map[string]interface{})
	m["name"] = s.Name
	m["passphrase"] = s.Passphrase
	m["is_custom"] = s.IsCustom

	if s.HorizonURL != "" {
		m["horizon_url"] = s.HorizonURL
	}
	if s.SorobanRPCURL != "" {
		m["soroban_rpc_url"] = s.SorobanRPCURL
	}
	if s.ProtocolVersion != 0 {
		m["protocol_version"] = s.ProtocolVersion
	}

	// Use json.Marshal on a sorted-key representation.
	data, _ := json.Marshal(m)
	// Re-marshal through sorted keys for full determinism.
	var sorted map[string]interface{}
	_ = json.Unmarshal(data, &sorted)

	// Sort keys and build canonical output.
	keys := make([]string, 0, len(sorted))
	for k := range sorted {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		keyBytes, _ := json.Marshal(k)
		valBytes, _ := json.Marshal(sorted[k])
		sb.Write(keyBytes)
		sb.WriteByte(':')
		sb.Write(valBytes)
	}
	sb.WriteByte('}')

	return []byte(sb.String())
}

// truncate shortens s to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
