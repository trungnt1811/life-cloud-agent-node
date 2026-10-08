package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// GovernanceDigest preserves integer precision and canonicalizes object keys.
func GovernanceDigest(schema string, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var object any
	if err = decoder.Decode(&object); err != nil {
		return "", err
	}
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(object); err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte(schema+"\n"), bytes.TrimSuffix(canonical.Bytes(), []byte("\n"))...))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
