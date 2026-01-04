// Package lockfile provides checksum handling for Gemfile.lock integrity verification.
package lockfile

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Checksum represents a gem integrity checksum.
// Ruby equivalent: Bundler::Checksum
type Checksum struct {
	Algorithm string // Checksum algorithm (e.g., "sha256", "sha512")
	Digest    string // Hex-encoded digest
}

const (
	// AlgoSeparator is the separator between algorithm and digest in lockfile format.
	AlgoSeparator = "="
	// DefaultAlgorithm is the default checksum algorithm used by Bundler.
	DefaultAlgorithm = "sha256"
)

var (
	// Regex for validating SHA256 hex digest (64 hex chars)
	sha256HexRegex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	// Regex for validating URL-safe base64 (43 chars + optional padding)
	base64Regex = regexp.MustCompile(`^[-0-9a-zA-Z_+/]{43}={0,2}$`)
)

// ToLock returns the lockfile representation of the checksum.
// Format: algo=digest (e.g., "sha256=abc123...")
func (c Checksum) ToLock() string {
	return c.Algorithm + AlgoSeparator + c.Digest
}

// String returns a human-readable representation of the checksum.
func (c Checksum) String() string {
	return fmt.Sprintf("%s:%s", c.Algorithm, c.Digest[:min(16, len(c.Digest))]+"...")
}

// ParseChecksum parses a checksum string from lockfile format.
// Supports both hex and URL-safe base64 encodings for SHA256.
func ParseChecksum(s string) (Checksum, error) {
	parts := strings.SplitN(s, AlgoSeparator, 2)
	if len(parts) != 2 {
		return Checksum{}, fmt.Errorf("invalid checksum format: %s", s)
	}

	algo := strings.ToLower(parts[0])
	digest := parts[1]

	// Convert to hex if needed (for SHA256)
	hexDigest, err := toHexDigest(digest, algo)
	if err != nil {
		return Checksum{}, err
	}

	return Checksum{
		Algorithm: algo,
		Digest:    hexDigest,
	}, nil
}

// toHexDigest converts a digest to hex format.
// Supports both hex and URL-safe base64 for SHA256.
func toHexDigest(digest, algo string) (string, error) {
	if algo != DefaultAlgorithm {
		// For non-SHA256, return as-is (assume it's already in correct format)
		return digest, nil
	}

	// Check if already hex
	if sha256HexRegex.MatchString(digest) {
		return strings.ToLower(digest), nil
	}

	// Try URL-safe base64
	if base64Regex.MatchString(digest) {
		// Convert URL-safe base64 to standard base64
		stdBase64 := strings.ReplaceAll(digest, "-", "+")
		stdBase64 = strings.ReplaceAll(stdBase64, "_", "/")

		decoded, err := base64.StdEncoding.DecodeString(stdBase64)
		if err != nil {
			return "", fmt.Errorf("invalid base64 checksum: %w", err)
		}
		return hex.EncodeToString(decoded), nil
	}

	return "", fmt.Errorf("invalid SHA256 digest format: %s", digest)
}

// parseChecksumString parses a comma-separated list of checksums.
// Format: algo1=digest1,algo2=digest2
func parseChecksumString(s string) []Checksum {
	if s == "" {
		return nil
	}

	var checksums []Checksum
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		checksum, err := ParseChecksum(part)
		if err != nil {
			// Skip invalid checksums (be permissive like Bundler)
			continue
		}
		checksums = append(checksums, checksum)
	}

	return checksums
}

// ChecksumsToLock formats checksums for lockfile output.
// Returns comma-separated checksums sorted by algorithm.
func ChecksumsToLock(checksums []Checksum) string {
	if len(checksums) == 0 {
		return ""
	}

	parts := make([]string, len(checksums))
	for i, c := range checksums {
		parts[i] = c.ToLock()
	}
	return strings.Join(parts, ",")
}
