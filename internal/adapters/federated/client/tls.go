package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TLSConfig configures the channel to the control center (decision 0006).
type TLSConfig struct {
	// Insecure opts out of TLS entirely. Only for local development and the
	// Phase 8 stub control center; every other field is ignored when set.
	Insecure bool
	// CAFile verifies the control center's certificate against a private CA
	// instead of the system pool. Blank uses the system pool.
	CAFile string
	// ClientCertFile/ClientKeyFile present this node's certificate (mTLS).
	// Setting exactly one is rejected by Credentials.
	ClientCertFile string
	ClientKeyFile  string
}

// Credentials builds the channel credentials for TLSConfig. It never returns
// a working plaintext credential unless Insecure is explicitly set.
func (c TLSConfig) Credentials() (credentials.TransportCredentials, error) {
	if c.Insecure {
		return insecure.NewCredentials(), nil
	}

	hasCert := strings.TrimSpace(c.ClientCertFile) != ""
	hasKey := strings.TrimSpace(c.ClientKeyFile) != ""
	if hasCert != hasKey {
		return nil, errors.New(
			"control center mTLS misconfigured: exactly one of CONTROL_CENTER_CLIENT_CERT_FILE/KEY_FILE is set; set both or neither",
		)
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if ca := strings.TrimSpace(c.CAFile); ca != "" {
		pem, err := os.ReadFile(ca)
		if err != nil {
			return nil, fmt.Errorf("read control center CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("control center CA file %q contains no usable certificates", ca)
		}
		tlsConfig.RootCAs = pool
	}

	if hasCert && hasKey {
		pair, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load control center client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{pair}
	}

	return credentials.NewTLS(tlsConfig), nil
}
