package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// TLSResult holds the TLS config and optional autocert manager.
type TLSResult struct {
	Config       *tls.Config
	AutocertMgr  *autocert.Manager // Non-nil when using Let's Encrypt
}

// SetupTLS returns a TLSResult using one of three strategies:
//  1. Let's Encrypt (autocert) when domain is non-empty
//  2. Provided cert/key files
//  3. Self-signed cert (generated to certDir on first run)
func SetupTLS(domain, certFile, keyFile, certDir string) (*TLSResult, error) {
	// Strategy 1: Let's Encrypt via autocert
	if domain != "" {
		log.Printf("tls: using Let's Encrypt for domain %q", domain)
		cacheDir := filepath.Join(certDir, "autocert-cache")
		if err := os.MkdirAll(cacheDir, 0700); err != nil {
			return nil, fmt.Errorf("creating autocert cache dir: %w", err)
		}
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domain),
			Cache:      autocert.DirCache(cacheDir),
		}
		return &TLSResult{Config: m.TLSConfig(), AutocertMgr: m}, nil
	}

	// Strategy 2: Provided cert/key
	if certFile != "" && keyFile != "" {
		log.Printf("tls: loading cert from %s, key from %s", certFile, keyFile)
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("loading TLS cert: %w", err)
		}
		return &TLSResult{Config: &tls.Config{Certificates: []tls.Certificate{cert}}}, nil
	}

	// Strategy 3: Self-signed
	log.Printf("tls: generating self-signed certificate in %s", certDir)
	cfg, err := generateSelfSigned(certDir)
	if err != nil {
		return nil, err
	}
	return &TLSResult{Config: cfg}, nil
}

// generateSelfSigned creates a self-signed certificate and saves it to certDir.
// If cert/key files already exist in certDir, they are loaded instead.
func generateSelfSigned(certDir string) (*tls.Config, error) {
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return nil, fmt.Errorf("creating cert dir: %w", err)
	}

	certPath := filepath.Join(certDir, "self-signed.crt")
	keyPath := filepath.Join(certDir, "self-signed.key")

	// Check if already exists
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			log.Printf("tls: loading existing self-signed cert from %s", certDir)
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, fmt.Errorf("loading existing self-signed cert: %w", err)
			}
			return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
		}
	}

	// Generate new cert
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"GoTinyMUSH"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, fmt.Errorf("creating certificate: %w", err)
	}

	// Write cert file
	certOut, err := os.Create(certPath)
	if err != nil {
		return nil, fmt.Errorf("writing cert: %w", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	// Write key file
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling key: %w", err)
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("writing key: %w", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyOut.Close()

	log.Printf("tls: self-signed cert written to %s", certDir)

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("loading generated cert: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}
