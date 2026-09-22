package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials/insecure"
)

func TestTLSConfigCredentials_InsecureOptOut(t *testing.T) {
	t.Parallel()

	creds, err := TLSConfig{Insecure: true}.Credentials()
	require.NoError(t, err)
	require.Equal(t, insecure.NewCredentials().Info(), creds.Info())
}

func TestTLSConfigCredentials_DefaultUsesSystemPool(t *testing.T) {
	t.Parallel()

	creds, err := TLSConfig{}.Credentials()
	require.NoError(t, err)
	require.NotEqual(t, insecure.NewCredentials().Info(), creds.Info())
}

func TestTLSConfigCredentials_RejectsOneSidedClientCert(t *testing.T) {
	t.Parallel()

	_, err := TLSConfig{ClientCertFile: "cert.pem"}.Credentials()
	require.ErrorContains(t, err, "exactly one of")

	_, err = TLSConfig{ClientKeyFile: "key.pem"}.Credentials()
	require.ErrorContains(t, err, "exactly one of")
}

func TestTLSConfigCredentials_RejectsUnreadableCAFile(t *testing.T) {
	t.Parallel()

	_, err := TLSConfig{CAFile: filepath.Join(t.TempDir(), "missing-ca.pem")}.Credentials()
	require.ErrorContains(t, err, "CA file")
}

func TestTLSConfigCredentials_RejectsCAFileWithoutCertificates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, []byte("not a certificate"), 0o600))

	_, err := TLSConfig{CAFile: caFile}.Credentials()
	require.ErrorContains(t, err, "no usable certificates")
}

func TestTLSConfigCredentials_LoadsValidCAAndClientCertificate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	caPEM, certPEM, keyPEM := generateTestCertificateChain(t)
	caFile := filepath.Join(dir, "ca.pem")
	certFile := filepath.Join(dir, "client-cert.pem")
	keyFile := filepath.Join(dir, "client-key.pem")
	require.NoError(t, os.WriteFile(caFile, caPEM, 0o600))
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0o600))

	creds, err := TLSConfig{
		CAFile:         caFile,
		ClientCertFile: certFile,
		ClientKeyFile:  keyFile,
	}.Credentials()
	require.NoError(t, err)
	require.NotNil(t, creds)
}

func TestTLSConfigCredentials_RejectsMismatchedCertAndKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, cert1PEM, _ := generateTestCertificateChain(t)
	_, _, key2PEM := generateTestCertificateChain(t)
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, cert1PEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, key2PEM, 0o600))

	_, err := TLSConfig{ClientCertFile: certFile, ClientKeyFile: keyFile}.Credentials()
	require.ErrorContains(t, err, "load control center client certificate")
}

// generateTestCertificateChain returns a self-signed CA-as-leaf certificate
// (PEM), reused as both "ca" and "client cert" since this test only checks
// that Credentials parses and loads what it is given, not chain validity.
func generateTestCertificateChain(t *testing.T) (caPEM, certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	caPEM = certPEM

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return caPEM, certPEM, keyPEM
}
