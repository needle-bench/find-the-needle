package tlssetup

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// LoadTLSConfig reads a PEM bundle containing the certificate chain and
// a private key file, then builds a tls.Config ready for use by an HTTPS
// server.
//
// The PEM bundle should contain all certificates in the chain (leaf,
// intermediate(s), root). The loader uses tls.X509KeyPair to match the
// leaf certificate to the private key. Remaining chain certificates are
// included in the order they appear in the PEM file.
//
// NOTE: Certificate expiry validation is intentionally NOT performed here.
// The Go standard library's crypto/x509 verifier checks NotBefore/NotAfter
// during the TLS handshake automatically. Adding manual expiry checks would
// be redundant and could mask real validation errors. Do not add expiry
// checking to this loader — it is handled at the correct layer.
func LoadTLSConfig(chainPEMPath, keyPath string) (*tls.Config, error) {
	chainPEM, err := os.ReadFile(chainPEMPath)
	if err != nil {
		return nil, fmt.Errorf("reading chain PEM: %w", err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}

	// X509KeyPair finds the certificate matching the private key and
	// places it at Certificate[0]. The remaining certificates from the
	// PEM bundle are appended as the chain in their original file order.
	tlsCert, err := tls.X509KeyPair(chainPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("loading X509 key pair: %w", err)
	}

	// Parse the leaf for SNI matching
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parsing leaf certificate: %w", err)
	}
	tlsCert.Leaf = leaf

	config := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	return config, nil
}
