package types

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
)

func (c GovernedProtectedCount) MarshalJSON() ([]byte, error) { return c.CanonicalJSON() }

// Persistence round trips retain disclosure only, never a suppressed raw count.
func (c *GovernedProtectedCount) UnmarshalJSON(encoded []byte) error {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &shape); err != nil || len(shape) != 2 || shape["count"] == nil || shape["effective_k"] == nil {
		return ErrInvalidProtectedCount
	}
	var disclosure map[string]json.RawMessage
	if err := json.Unmarshal(shape["count"], &disclosure); err != nil || len(disclosure) != 4 || disclosure["kind"] == nil || disclosure["value"] == nil || disclosure["lower_bound"] == nil || disclosure["upper_bound"] == nil {
		return ErrInvalidProtectedCount
	}
	var payload struct {
		Count *struct {
			Kind       string  `json:"kind"`
			Value      *string `json:"value"`
			LowerBound *string `json:"lower_bound"`
			UpperBound *string `json:"upper_bound"`
		} `json:"count"`
		EffectiveK string `json:"effective_k"`
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || payload.Count == nil {
		return ErrInvalidProtectedCount
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ErrInvalidProtectedCount
	}
	k, err := protectedDecimal(payload.EffectiveK)
	if err != nil {
		return err
	}
	var count GovernedProtectedCount
	switch payload.Count.Kind {
	case "EXACT":
		if payload.Count.Value == nil || payload.Count.LowerBound != nil || payload.Count.UpperBound != nil {
			return ErrInvalidProtectedCount
		}
		value, parseErr := protectedDecimal(*payload.Count.Value)
		if parseErr != nil {
			return parseErr
		}
		count, err = NewGovernedExactCount(value, k)
	case "SUPPRESSED":
		if payload.Count.Value != nil || payload.Count.LowerBound == nil || payload.Count.UpperBound == nil || *payload.Count.LowerBound != "1" || *payload.Count.UpperBound != strconv.FormatUint(k-1, 10) {
			return ErrInvalidProtectedCount
		}
		count, err = NewGovernedSuppressedCount(k)
	default:
		return ErrInvalidProtectedCount
	}
	if err != nil {
		return err
	}
	*c = count
	return nil
}

func protectedDecimal(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(parsed, 10) != value {
		return 0, ErrInvalidProtectedCount
	}
	return parsed, nil
}
