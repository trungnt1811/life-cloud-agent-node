package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"
)

const ProtectedPayloadHashSchema = "protected-cohort-payload/v1"

var ErrInvalidProtectedCount = errors.New("protected cohort count is invalid")

// Only approved disclosure is retained. Suppressed counts contain no raw value.
type GovernedProtectedCount struct {
	effectiveK uint64
	value      uint64
	exact      bool
}

func NewGovernedExactCount(value, effectiveK uint64) (GovernedProtectedCount, error) {
	if !validGovernedK(effectiveK) || value > math.MaxInt64 || (value > 0 && value < effectiveK) {
		return GovernedProtectedCount{}, ErrInvalidProtectedCount
	}
	return GovernedProtectedCount{effectiveK: effectiveK, value: value, exact: true}, nil
}

func NewGovernedSuppressedCount(effectiveK uint64) (GovernedProtectedCount, error) {
	if !validGovernedK(effectiveK) {
		return GovernedProtectedCount{}, ErrInvalidProtectedCount
	}
	return GovernedProtectedCount{effectiveK: effectiveK}, nil
}

func validGovernedK(k uint64) bool { return k >= GovernedNetworkFloor && k <= math.MaxInt64 }

func (c GovernedProtectedCount) EffectiveK() uint64 { return c.effectiveK }

func (c GovernedProtectedCount) ExactValue() (uint64, bool) {
	return c.value, c.exact && validGovernedK(c.effectiveK)
}

func (c GovernedProtectedCount) Bounds() (uint64, uint64, bool) {
	if c.exact || !validGovernedK(c.effectiveK) {
		return 0, 0, false
	}
	return 1, c.effectiveK - 1, true
}

func (c GovernedProtectedCount) canonicalPayload() (map[string]any, error) {
	if !validGovernedK(c.effectiveK) {
		return nil, ErrInvalidProtectedCount
	}
	disclosure := map[string]any{"kind": "SUPPRESSED", "value": nil, "lower_bound": "1", "upper_bound": strconv.FormatUint(c.effectiveK-1, 10)}
	if c.exact {
		disclosure = map[string]any{"kind": "EXACT", "value": strconv.FormatUint(c.value, 10), "lower_bound": nil, "upper_bound": nil}
	}
	return map[string]any{"count": disclosure, "effective_k": strconv.FormatUint(c.effectiveK, 10)}, nil
}

func (c GovernedProtectedCount) CanonicalJSON() ([]byte, error) {
	payload, err := c.canonicalPayload()
	if err != nil {
		return nil, err
	}
	return json.Marshal(payload)
}

func (c GovernedProtectedCount) Digest() (string, error) {
	encoded, err := c.CanonicalJSON()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte(ProtectedPayloadHashSchema+"\n"), encoded...))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

const GovernedNetworkFloor uint64 = 10

// ProtectGovernedCount is not the legacy k=5 path. It is used only by governed work.
func ProtectGovernedCount(raw, localK uint64) (GovernedProtectedCount, error) {
	if localK == 0 || localK > math.MaxInt64 || raw > math.MaxInt64 {
		return GovernedProtectedCount{}, ErrInvalidProtectedCount
	}
	effectiveK := max(GovernedNetworkFloor, localK)
	if raw > 0 && raw < effectiveK {
		return NewGovernedSuppressedCount(effectiveK)
	}
	return NewGovernedExactCount(raw, effectiveK)
}
