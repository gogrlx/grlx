package certs

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/gogrlx/grlx/v2/internal/config"
	log "github.com/gogrlx/grlx/v2/internal/log"
)

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	default:
		return nil
	}
}

var notBefore = time.Now()

func genCACert() error {
	RootCAPriv := config.RootCAPriv
	RootCA := config.RootCA
	_, err := os.Stat(RootCAPriv)
	if !os.IsNotExist(err) {
		_, err = os.Stat(RootCA)
		if !os.IsNotExist(err) {
			log.Trace("Found a RootCA keypair, not generating a new one...")
			return nil
		}
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}
	caCert := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{config.FarmerOrganization},
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(config.CertificateValidTime),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	caCert.IsCA = true
	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %w", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, &caCert, &caCert, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	certOut, err := os.Create(RootCA)
	if err != nil {
		return fmt.Errorf("failed to create CA cert file %s: %w", RootCA, err)
	}
	if err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes}); err != nil {
		certOut.Close()
		return fmt.Errorf("failed to encode CA cert PEM: %w", err)
	}
	if err = certOut.Close(); err != nil {
		return fmt.Errorf("failed to close CA cert file: %w", err)
	}
	log.Debugf("wrote %s", RootCA)

	privBytes, err := x509.MarshalPKCS8PrivateKey(caPrivKey)
	if err != nil {
		return fmt.Errorf("unable to marshal CA private key: %w", err)
	}
	keyOut, err := os.OpenFile(RootCAPriv, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create CA key file %s: %w", RootCAPriv, err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		keyOut.Close()
		return fmt.Errorf("failed to encode CA private key PEM: %w", err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("failed to close CA key file: %w", err)
	}
	return nil
}

func GenCert() error {
	CertFile := config.CertFile
	KeyFile := config.KeyFile
	RootCA := config.RootCA
	_, err := os.Stat(CertFile)
	if !os.IsNotExist(err) {
		_, err = os.Stat(KeyFile)
		if !os.IsNotExist(err) {
			log.Trace("Found a TLS keypair, not generating a new one...")
			return nil
		}
	}

	if err := genCACert(); err != nil {
		return fmt.Errorf("failed to generate CA cert: %w", err)
	}

	caCertBytes, err := os.ReadFile(RootCA)
	if err != nil {
		return fmt.Errorf("failed to read CA cert %s: %w", RootCA, err)
	}
	block, _ := pem.Decode(caCertBytes)
	if block == nil {
		return fmt.Errorf("failed to decode CA cert PEM from %s", RootCA)
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	rootCAPrivBytes, err := os.ReadFile(config.RootCAPriv)
	if err != nil {
		return fmt.Errorf("failed to read CA private key %s: %w", config.RootCAPriv, err)
	}
	block2, _ := pem.Decode(rootCAPrivBytes)
	if block2 == nil {
		return fmt.Errorf("failed to decode CA private key PEM from %s", config.RootCAPriv)
	}
	caPriv, err := x509.ParsePKCS8PrivateKey(block2.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA private key: %w", err)
	}

	hosts := config.CertHosts
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{config.FarmerOrganization},
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(config.CertificateValidTime),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}
	template.IsCA = false

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, publicKey(priv), caPriv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}
	certOut, err := os.Create(CertFile)
	if err != nil {
		return fmt.Errorf("failed to create cert file %s: %w", CertFile, err)
	}
	if err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		certOut.Close()
		return fmt.Errorf("failed to encode cert PEM: %w", err)
	}
	if err = certOut.Close(); err != nil {
		return fmt.Errorf("failed to close cert file: %w", err)
	}
	log.Debug("wrote cert.pem")

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("unable to marshal private key: %w", err)
	}
	keyOut, err := os.OpenFile(KeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create key file %s: %w", KeyFile, err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		keyOut.Close()
		return fmt.Errorf("failed to encode private key PEM: %w", err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("failed to close key file: %w", err)
	}
	log.Debug("wrote key.pem")
	return nil
}

// RotateTLSCerts checks the server certificate's expiration and regenerates
// it (signed by the existing CA) if it expires within the given threshold.
// The CA certificate is not rotated — only the leaf server cert and key.
// Returns true if the certificate was rotated.
func RotateTLSCerts(threshold time.Duration) (bool, error) {
	certBytes, err := os.ReadFile(config.CertFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No cert exists — generate fresh.
			if err := GenCert(); err != nil {
				return false, fmt.Errorf("failed to generate cert: %w", err)
			}
			return true, nil
		}
		return false, fmt.Errorf("failed to read cert file: %w", err)
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		// Corrupt PEM — regenerate.
		log.Warn("TLS certificate PEM is corrupt, regenerating")
		return forceRegenCert()
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Warnf("Failed to parse TLS certificate: %v, regenerating", err)
		return forceRegenCert()
	}

	remaining := time.Until(cert.NotAfter)
	if remaining > threshold {
		log.Tracef("TLS certificate valid for %s, rotation threshold is %s — no rotation needed",
			remaining.Round(time.Minute), threshold.Round(time.Minute))
		return false, nil
	}

	log.Infof("TLS certificate expires in %s (threshold %s), rotating",
		remaining.Round(time.Minute), threshold.Round(time.Minute))
	return forceRegenCert()
}

// forceRegenCert removes the existing server cert and key, then regenerates them.
func forceRegenCert() (bool, error) {
	// Remove existing cert and key so GenCert will regenerate.
	if err := os.Remove(config.CertFile); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to remove old cert: %w", err)
	}
	if err := os.Remove(config.KeyFile); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to remove old key: %w", err)
	}
	if err := GenCert(); err != nil {
		return false, fmt.Errorf("failed to regenerate cert: %w", err)
	}
	return true, nil
}
