package certs

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/gogrlx/grlx/v2/internal/config"
	log "github.com/taigrr/log-socket/log"
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

func genCACert() {
	RootCAPriv := config.RootCAPriv
	RootCA := config.RootCA
	_, err := os.Stat(RootCAPriv)
	if !os.IsNotExist(err) {
		_, err = os.Stat(RootCAPriv)
		if !os.IsNotExist(err) {
			log.Trace("Found a RootCA keypair, not generating a new one...")
			return
		}
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Panicf("Failed to generate serial number: %v", err)
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
		log.Panic(err.Error())
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, &caCert, &caCert, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		log.Panic(err.Error())
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	certOut, err := os.Create(RootCA)
	if err != nil {
		log.Fatalf("%v", err)
	}
	if err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes}); err != nil {
		log.Fatalf("%v", err)
	}
	if err = certOut.Close(); err != nil {
		log.Fatalf("%v", err)
	}
	log.Debugf("wrote %s", RootCA)
	keyOut, err := os.OpenFile(RootCAPriv, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("%v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(caPrivKey)
	if err != nil {
		log.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		log.Fatalf("Failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("Error closing key.pem: %v", err)
	}
}

func GenCert() {
	// check if certificates already exist first
	CertFile := config.CertFile
	KeyFile := config.KeyFile
	RootCA := config.RootCA
	_, err := os.Stat(CertFile)
	if !os.IsNotExist(err) {
		_, err = os.Stat(KeyFile)
		if !os.IsNotExist(err) {
			log.Trace("Found a TLS keypair, not generating a new one...")
			return
		}
	}

	genCACert()
	file, err := os.Open(RootCA)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()
	stats, statsErr := file.Stat()
	if statsErr != nil {
		log.Panic(err)
	}
	size := stats.Size()
	bytes := make([]byte, size)
	bufr := bufio.NewReader(file)
	_, err = bufr.Read(bytes)
	if err != nil {
		log.Panic("could not read rootCA file into buffer", err)
	}
	block, _ := pem.Decode(bytes)
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Panic(err.Error())
	}
	rootCAPrivFile, err := os.Open(config.RootCAPriv)
	if err != nil {
		log.Panic(err)
	}
	defer rootCAPrivFile.Close()
	stats, statsErr = rootCAPrivFile.Stat()
	if statsErr != nil {
		log.Panic(err)
	}
	size = stats.Size()
	rootCAPrivBytes := make([]byte, size)
	bufr2 := bufio.NewReader(rootCAPrivFile)
	_, err = bufr2.Read(rootCAPrivBytes)
	if err != nil {
		log.Panic("could not read rootCA private key file into buffer", err)
	}
	block2, _ := pem.Decode(rootCAPrivBytes)
	caPriv, err := x509.ParsePKCS8PrivateKey(block2.Bytes)
	if err != nil {
		log.Panic(err.Error())
	}
	hosts := config.CertHosts
	var priv interface{}
	priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Panicf("Failed to generate private key: %v", err)
	}
	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	keyUsage |= x509.KeyUsageCertSign
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
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
		log.Fatalf("Failed to create certificate: %v", err)
	}
	certOut, err := os.Create(CertFile)
	if err != nil {
		log.Fatalf("Failed to open cert.pem for writing: %v", err)
	}
	if err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to cert.pem: %v", err)
	}
	if err = certOut.Close(); err != nil {
		log.Fatalf("Error closing cert.pem: %v", err)
	}
	log.Debug("wrote cert.pem")
	keyOut, err := os.OpenFile(KeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("Failed to open key.pem for writing: %v", err)
		return
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		log.Fatalf("Failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("Error closing key.pem: %v", err)
	}
	log.Debug("wrote key.pem")
}

// TODO: add TLS cert rotation
func RotateTLSCerts() {
}
