/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-24 10:22:25
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:11:03
 * @FilePath: /MLC_GO/pkg/security/security_v01/security_mtls_tool.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package securityV01

import (
	"MLC_GO/internal/pkg/logHG"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ---- Types used for app-layer payloads ----
type EncryptedMessage struct {
	EncKey     string `json:"enc_key"`     // base64 of RSA-OAEP(encrypted AES key)
	Nonce      string `json:"nonce"`       // base64 nonce for AES-GCM
	Ciphertext string `json:"ciphertext"`  // base64 AES-GCM ciphertext
}

var tlsCfgAtomic atomic.Value // will store *tls.Config

// ------------------ Utilities: file write/read helpers ------------------

func mustWriteFile(path string, data []byte, perm os.FileMode) {
	if err := os.WriteFile(path, data, perm); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}

// ------------------ Cert generation (RSA or ECDSA) ------------------

// generateRSAKey creates RSA private key
func generateRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// generateECDSAKey creates ECDSA private key (P-256)
func generateECDSAKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// create self-signed CA and sign server+client certs
// if useECDSA==true => generate ECDSA keys & certs for TLS signing (note: ECDSA keys cannot be used as RSA encryption keys)
func genCerts(useECDSA bool) error {
	log.Printf("Generating certs (useECDSA=%v)...", useECDSA)

	// 1) Generate CA (self-signed)
	var caPriv interface{}
	var caPrivPEM []byte
	var caCertDER []byte
	serial := big.NewInt(1)
	now := time.Now()
	if useECDSA {
		pk, err := generateECDSAKey()
		if err != nil { return err }
		caPriv = pk
		caPrivDER, _ := x509.MarshalECPrivateKey(pk)
		caPrivPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caPrivDER})

		caT := &x509.Certificate{
			SerialNumber: serial,
			Subject: pkix.Name{Organization: []string{"Example CA Org"}, CommonName: "Example-Root-CA"},
			NotBefore: now.Add(-time.Hour),
			NotAfter:  now.Add(365*24*time.Hour),
			KeyUsage:  x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			IsCA:      true, BasicConstraintsValid: true,
		}
		caCertDER, _ = x509.CreateCertificate(rand.Reader, caT, caT, &pk.PublicKey, pk)
	} else {
		pk, err := generateRSAKey()
		if err != nil { return err }
		caPriv = pk
		caPrivPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})

		caT := &x509.Certificate{
			SerialNumber: serial,
			Subject:      pkix.Name{Organization: []string{"Example CA Org"}, CommonName: "Example-Root-CA"},
			NotBefore:    now.Add(-time.Hour),
			NotAfter:     now.Add(365*24*time.Hour),
			KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			IsCA:         true, BasicConstraintsValid: true,
		}
		caCertDER, _ = x509.CreateCertificate(rand.Reader, caT, caT, publicKeyOf(caPriv), caPriv)
	}

	mustWriteFile("ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}), 0644)
	mustWriteFile("ca.key", caPrivPEM, 0600)
	log.Println("  wrote ca.pem, ca.key")

	// Helper to create and write a cert signed by CA
	createAndWrite := func(name string, isServer bool) error {
		serial := big.NewInt(time.Now().UnixNano())
		if useECDSA {
			priv, err := generateECDSAKey()
			if err != nil { return err }
			privDER, _ := x509.MarshalECPrivateKey(priv)
			privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

			tmpl := &x509.Certificate{
				SerialNumber: serial,
				Subject:      pkix.Name{Organization: []string{"Example Org"}, CommonName: name},
				NotBefore:    now.Add(-time.Hour),
				NotAfter:     now.Add(365*24*time.Hour),
				KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				BasicConstraintsValid: true,
			}
			if isServer {
				tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
				tmpl.DNSNames = []string{"localhost"}
			} else {
				tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			}
			der, err := x509.CreateCertificate(rand.Reader, tmpl, parseCertFromPEM("ca.pem"), &priv.PublicKey, caPriv)
			if err != nil { return err }
			certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
			mustWriteFile(fmt.Sprintf("%s.pem", name), certPEM, 0644)
			mustWriteFile(fmt.Sprintf("%s.key", name), privPEM, 0600)
		} else {
			priv, err := generateRSAKey()
			if err != nil { return err }
			privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

			tmpl := &x509.Certificate{
				SerialNumber: serial,
				Subject:      pkix.Name{Organization: []string{"Example Org"}, CommonName: name},
				NotBefore:    now.Add(-time.Hour),
				NotAfter:     now.Add(365*24*time.Hour),
				KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				BasicConstraintsValid: true,
			}
			if isServer {
				tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
				tmpl.DNSNames = []string{"localhost"}
			} else {
				tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			}
			der, err := x509.CreateCertificate(rand.Reader, tmpl, parseCertFromPEM("ca.pem"), publicKeyOf(priv), caPriv)
			if err != nil { return err }
			certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
			mustWriteFile(fmt.Sprintf("%s.pem", name), certPEM, 0644)
			mustWriteFile(fmt.Sprintf("%s.key", name), privPEM, 0600)
		}
		return nil
	}

	if err := createAndWrite("server", true); err != nil { return err }
	if err := createAndWrite("client", false); err != nil { return err }

	log.Println("Generated server.pem/server.key and client.pem/client.key")
	return nil
}

// parseCertFromPEM convenience: returns *x509.Certificate from file path (used for CA)
func parseCertFromPEM(path string) *x509.Certificate {
	bs, err := os.ReadFile(path)
	if err != nil { log.Fatalf("read cert %s: %v", path, err) }
	block, _ := pem.Decode(bs)
	if block == nil { log.Fatalf("no PEM in %s", path) }
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil { log.Fatalf("parse cert %s: %v", path, err) }
	return cert
}

// publicKeyOf returns public key for rsa or ecdsa private key
func publicKeyOf(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

// ------------------ TLS config loading & hot-reload ------------------

// loadTLSConfig constructs *tls.Config using server.pem/server.key and ca.pem
func loadTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair("server.pem", "server.key")
	if err != nil { return nil, fmt.Errorf("load server keypair: %w", err) }
	caPEM, err := os.ReadFile("ca.pem")
	if err != nil { return nil, fmt.Errorf("read ca.pem: %w", err) }
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caPEM); !ok { return nil, fmt.Errorf("append ca.pem failed") }

	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}
	return cfg, nil
}

// watchCertFiles listens for changes to cert/key/ca and reloads
func watchCertFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil { log.Fatalf("fsnotify.NewWatcher: %v", err) }
	defer watcher.Close()
	if err := watcher.Add("."); err != nil { log.Fatalf("watcher.Add: %v", err) }
	for {
		select {
		case ev := <-watcher.Events:
			if ev.Op&(fsnotify.Write|fsnotify.Create) != 0 &&
				(ev.Name == "server.pem" || ev.Name == "server.key" || ev.Name == "ca.pem") {
				log.Printf("cert change detected: %s. reloading...\n", ev.Name)
				if cfg, err := loadTLSConfig(); err == nil {
					tlsCfgAtomic.Store(cfg)
					log.Println("tls config reloaded")
				} else {
					log.Printf("reload failed: %v\n", err)
				}
			}
		case err := <-watcher.Errors:
			log.Printf("watcher error: %v\n", err)
		}
	}
}

// ------------------ AES-GCM helpers (app-layer) ------------------

func aesGCMEncrypt(aesKey, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil { return nil, nil, err }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return nil, nil, err }
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return nil, nil, err }
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return ct, nonce, nil
}

func aesGCMDecrypt(aesKey, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil { return nil, err }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return nil, err }
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ------------------ Server handler: decrypt -> process -> encrypt response ------------------

func serverHandler(w http.ResponseWriter, r *http.Request) {
	// check TLS + peer cert
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		http.Error(w, "client certificate required", http.StatusUnauthorized)
		return
	}
	var in EncryptedMessage
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	// decode
	encKeyBytes, err := base64.StdEncoding.DecodeString(in.EncKey)
	if err != nil { http.Error(w, "invalid enc_key", http.StatusBadRequest); return }
	nonce, err := base64.StdEncoding.DecodeString(in.Nonce)
	if err != nil { http.Error(w, "invalid nonce", http.StatusBadRequest); return }
	ct, err := base64.StdEncoding.DecodeString(in.Ciphertext)
	if err != nil { http.Error(w, "invalid ciphertext", http.StatusBadRequest); return }

	// decrypt AES key with server private key
	serverPriv, err := loadRSAPrivateKey("server.key")
	if err != nil { http.Error(w, "server key error", http.StatusInternalServerError); return }
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, serverPriv, encKeyBytes, nil)
	if err != nil { http.Error(w, "rsa decrypt failed", http.StatusInternalServerError); return }

	plain, err := aesGCMDecrypt(aesKey, nonce, ct)
	if err != nil { http.Error(w, "aes decrypt failed", http.StatusInternalServerError); return }
	log.Printf("server: got plain: %s\n", string(plain))

	// process: append text
	respPlain := []byte(string(plain) + " + server-data")

	// prepare response encryption: use client's public key (from TLS peer cert)
	clientCert := r.TLS.PeerCertificates[0]
	clientPub, ok := clientCert.PublicKey.(*rsa.PublicKey)
	if !ok { http.Error(w, "client pub not RSA", http.StatusInternalServerError); return }

	// generate response AES key
	respAES := make([]byte, 32)
	if _, err := rand.Read(respAES); err != nil { http.Error(w, "rand failed", http.StatusInternalServerError); return }
	respCT, respNonce, err := aesGCMEncrypt(respAES, respPlain)
	if err != nil { http.Error(w, "aes encrypt failed", http.StatusInternalServerError); return }

	// encrypt respAES with client's RSA pub
	encRespKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, clientPub, respAES, nil)
	if err != nil { http.Error(w, "rsa encrypt resp key failed", http.StatusInternalServerError); return }

	out := EncryptedMessage{
		EncKey:     base64.StdEncoding.EncodeToString(encRespKey),
		Nonce:      base64.StdEncoding.EncodeToString(respNonce),
		Ciphertext: base64.StdEncoding.EncodeToString(respCT),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// loadRSAPrivateKey reads a PEM RSA private key (PKCS1 or PKCS8)
func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	bs, err := os.ReadFile(path)
	if err != nil { return nil, err }
	block, _ := pem.Decode(bs)
	if block == nil { return nil, fmt.Errorf("no PEM data") }
	// try PKCS1
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil { return key, nil }
	// try PKCS8
	if k2, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if key, ok := k2.(*rsa.PrivateKey); ok { return key, nil }
	}
	return nil, fmt.Errorf("unsupported private key format")
}

// ------------------ Client logic: build payload, send request, decrypt response ------------------

func runClient() error {
	// load client cert/key for mTLS
	tpair, err := tls.LoadX509KeyPair("client.pem", "client.key")
	if err != nil { return fmt.Errorf("load client keypair: %w", err) }
	caPEM, err := os.ReadFile("ca.pem")
	if err != nil { return fmt.Errorf("read ca: %w", err) }
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caPEM); !ok { return fmt.Errorf("append ca failed") }

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{tpair},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}

	tr := &http.Transport{TLSClientConfig: tlsCfg}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	plain := []byte("hello-from-client")

	// AES key & encrypt
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil { return fmt.Errorf("rand aes: %w", err) }
	ct, nonce, err := aesGCMEncrypt(aesKey, plain)
	if err != nil { return fmt.Errorf("aes encrypt: %w", err) }

	// load server public RSA key from server.pem
	serverPub, err := loadRSAPubFromCert("server.pem")
	if err != nil { return fmt.Errorf("load server pub: %w", err) }
	encKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, serverPub, aesKey, nil)
	if err != nil { return fmt.Errorf("rsa encrypt aes: %w", err) }

	msg := EncryptedMessage{
		EncKey:     base64.StdEncoding.EncodeToString(encKey),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
	}
	bs, _ := json.Marshal(msg)
	resp, err := client.Post("https://localhost:8443/api/process", "application/json", bytes.NewReader(bs))
	if err != nil { return fmt.Errorf("post: %w", err) }
	defer resp.Body.Close()

	var out EncryptedMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return fmt.Errorf("decode resp: %w", err) }

	// decrypt response AES key using client private key
	clientPriv, err := loadRSAPrivateKey("client.key")
	if err != nil { return fmt.Errorf("load client priv: %w", err) }
	encRespKey, _ := base64.StdEncoding.DecodeString(out.EncKey)
	respAESKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, clientPriv, encRespKey, nil)
	if err != nil { return fmt.Errorf("decrypt resp key: %w", err) }
	respNonce, _ := base64.StdEncoding.DecodeString(out.Nonce)
	respCT, _ := base64.StdEncoding.DecodeString(out.Ciphertext)
	plainResp, err := aesGCMDecrypt(respAESKey, respNonce, respCT)
	if err != nil { return fmt.Errorf("aes decrypt resp: %w", err) }

	fmt.Println("Client received plain response:", string(plainResp))
	return nil
}

// load RSA public key from cert PEM
func loadRSAPubFromCert(path string) (*rsa.PublicKey, error) {
	bs, err := os.ReadFile(path)
	if err != nil { return nil, err }
	block, _ := pem.Decode(bs)
	if block == nil { return nil, fmt.Errorf("no PEM") }
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil { return nil, err }
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok { return nil, fmt.Errorf("pub not RSA") }
	return pub, nil
}

// ------------------ Main: flags and control flow ------------------

func SecurityV01Main() {
	genFlag := flag.Bool("gen", false, "generate certs (ca.pem/ca.key, server.pem/server.key, client.pem/client.key)")
	serverFlag := flag.Bool("server", false, "run server")
	clientFlag := flag.Bool("client", false, "run client (one request)")
	useECDSA := flag.Bool("ecdsa", false, "when generating, create ECDSA certs (for TLS signing). Note: ECDSA keys do not do RSA encryption")
	flag.Parse()

	if *genFlag {
		if err := genCerts(*useECDSA); err != nil {
			logHG.ErrFInfo("gen certs failed: %v", err)
		}
		return
	}
	if *serverFlag {
		// initial load
		cfg, err := loadTLSConfig()
		if err != nil { log.Fatalf("initial load tls config: %v", err) }
		tlsCfgAtomic.Store(cfg)
		// wrapper that returns newest config for each handshake
		wrapper := &tls.Config{
			GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
				return tlsCfgAtomic.Load().(*tls.Config), nil
			},
			MinVersion: tls.VersionTLS12,
		}
		// start watcher
		go watchCertFiles()
		http.HandleFunc("/api/process", serverHandler)
		srv := &http.Server{
			Addr:      ":8443",
			TLSConfig: wrapper,
			ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
		}
		log.Println("HTTPS mTLS server listening https://localhost:8443")
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			logHG.ErrFInfo("server exit: %v", err)
		}
		return
	}
	if *clientFlag {
		if err := runClient(); err != nil {
			logHG.ErrFInfo("client failed: %v", err)
		}
		return
	}

	flag.Usage()
}

