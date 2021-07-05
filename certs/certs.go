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
	"math/big"
	"net"
	"os"
	"time"

	. "github.com/gogrlx/grlx/config"
	"github.com/nats-io/nkeys"
	log "github.com/taigrr/log-socket/logger"
)

var ()

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

var notBefore time.Time
var Organization string

func GenCert(hostnames []string) {
	hosts := []string{"localhost", "127.0.0.1", "farmer"}
	var priv interface{}
	var err error
	priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}
	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	keyUsage |= x509.KeyUsageCertSign
	notBefore = time.Now()
	validFor := CertificateValidTime
	notAfter := notBefore.Add(validFor)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{Organization},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	hosts = append(hosts, hostnames...)
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}
	template.IsCA = true

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}
	certOut, err := os.Create(CertFile)
	if err != nil {
		log.Fatalf("Failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		log.Fatalf("Error closing cert.pem: %v", err)
	}
	log.Debug("wrote cert.pem")
	keyOut, err := os.OpenFile(KeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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
func GenNKey() {
	_, err := os.Stat(NKeyPrivFile)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		kp, err := nkeys.CreateUser()
		if err != nil {
			log.Panic(err.Error())
		}
		pubKey, err := os.OpenFile(NKeyPubFile,
			os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
			0600,
		)
		if err != nil {
			log.Panic(err.Error())
		}
		defer pubKey.Close()
		key, err := kp.PublicKey()
		_, err = pubKey.Write([]byte(key))
		if err != nil {
			log.Panic(err.Error())
		}

		privKey, err := os.OpenFile(NKeyPrivFile,
			os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
			0600,
		)
		if err != nil {
			log.Panic(err.Error())
		}
		defer privKey.Close()
		pkey, err := kp.Seed()
		_, err = privKey.Write(pkey)
		if err != nil {
			log.Panic(err.Error())
		}
		return
	}
	log.Panic(err)

}
