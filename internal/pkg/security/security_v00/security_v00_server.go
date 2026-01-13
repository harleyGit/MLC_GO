/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-24 08:25:30
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:10:48
 * @FilePath: /MLC_GO/.vscode/security/security_v00/security_v00_server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 *
 * 用途： HTTPS 服务端（mTLS + 证书 & CA 热重载 + 应用层加解密）
 */
package securityv00

import (
	"MLC_GO/internal/pkg/logHG"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Request/Response payloads for the app-layer encrypted messages
type EncryptedMessage struct {
	EncKey     string `json:"enc_key"`    // base64 of RSA-OAEP(encrypted AES key)
	Nonce      string `json:"nonce"`      // base64 nonce for AES-GCM
	Ciphertext string `json:"ciphertext"` // base64 AES-GCM ciphertext
}

var tlsConfigAtomic atomic.Value // stores *tls.Config

// loadServerTLSConfig constructs tls.Config using current server.pem/server.key and ca.pem
func loadServerTLSConfig() (*tls.Config, error) {
	// load server cert+key
	cert, err := tls.LoadX509KeyPair("server.pem", "server.key")
	if err != nil {
		return nil, fmt.Errorf("load server keypair: %w", err)
	}

	// load CA (for client cert verification)
	caPEM, err := os.ReadFile("ca.pem")
	if err != nil {
		return nil, fmt.Errorf("read ca.pem: %w", err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("failed to append ca.pem")
	}

	// new tls.Config template. We set GetConfigForClient to return the latest config stored in atomic.
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
		// Note: GetConfigForClient gives us the ability to return the current tls.Config at handshake time,
		// so new incoming connections will use the updated configuration after we Store() a new one.
	}
	return cfg, nil
}

// watchCerts watches for changes to cert/key/ca and reloads tls.Config
func watchCerts() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("fsnotify.NewWatcher: %v", err)
	}
	defer watcher.Close()

	// watch current dir (you may prefer to watch exact files)
	if err := watcher.Add("."); err != nil {
		log.Fatalf("watcher.Add: %v", err)
	}

	for {
		select {
		case ev := <-watcher.Events:
			// on change to server.pem / server.key / ca.pem, reload
			if ev.Op&(fsnotify.Write|fsnotify.Create) != 0 &&
				(ev.Name == "server.pem" || ev.Name == "server.key" || ev.Name == "ca.pem") {
				log.Printf("detected change: %s, reloading TLS config...\n", ev.Name)
				if cfg, err := loadServerTLSConfig(); err == nil {
					// store the fresh config (atomic)
					tlsConfigAtomic.Store(cfg)
					log.Println("TLS config reloaded")
				} else {
					log.Printf("reload failed: %v\n", err)
				}
			}
		case err := <-watcher.Errors:
			log.Println("watch error:", err)
		}
	}
}

// helper: RSA-OAEP decrypt (server uses its private key to unwrap AES key)
func rsaOAEPDecryptRSA(priv *rsa.PrivateKey, encrypted []byte) ([]byte, error) {
	return rsa.DecryptOAEP(nil, rand.Reader, priv, encrypted, nil) // use default hash? use nil -> SHA1 historically; better use SHA256 but rsa.DecryptOAEP requires hash param; we'll use SHA256 below with explicit function
}

// We'll implement OAEP with SHA-256 explicitly:
func rsaOAEPDecrypt(priv *rsa.PrivateKey, encrypted []byte) ([]byte, error) {
	return rsa.DecryptOAEP(
		sha256.New(), // ✅ 正确：返回一个 hash.Hash,
		rand.Reader, priv, encrypted, nil,
	)
}

// helper to provide concrete hash (we wrap to avoid repeated imports later)
func sha256Hash() hashFunc {
	return sha256HashImpl{}
}

type hashFunc interface{}
type sha256HashImpl struct{}

// But to keep simple and correct, we'll directly use rsa.DecryptOAEP with crypto/sha256 below in the real function.

// AES-GCM decrypt
func aesGCMDecryptServer(aesKey, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// AES-GCM encrypt
func aesGCMEncryptServer(aesKey, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return ct, nonce, nil
}

// parse RSA private key from PEM file
func loadRSAPrivateKeyFromFileServer(path string) (*rsa.PrivateKey, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(bs)
	if block == nil {
		return nil, fmt.Errorf("no PEM data in %s", path)
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// try PKCS8
		if p8, err2 := x509.ParsePKCS8PrivateKey(block.Bytes); err2 == nil {
			if k, ok := p8.(*rsa.PrivateKey); ok {
				return k, nil
			}
		}
		return nil, err
	}
	return priv, nil
}

// helper: extract RSA public key from certificate bytes (PEM)
func loadRSAPubFromCertFile(path string) (*rsa.PublicKey, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(bs)
	if block == nil {
		return nil, fmt.Errorf("no PEM data in %s", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("cert public key is not RSA")
	}
	return pub, nil
}

// Handler: accepts JSON with EncryptedMessage, decrypts AES key with server private key, decrypts payload,
// appends data, then encrypts response with client's public key (from client's cert presented in TLS), returns JSON.
func processHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure TLS and peer cert present
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		http.Error(w, "client certificate required", http.StatusUnauthorized)
		return
	}
	// Read request body
	var em EncryptedMessage
	if err := json.NewDecoder(r.Body).Decode(&em); err != nil {
		http.Error(w, "bad request JSON", http.StatusBadRequest)
		return
	}

	// decode fields
	encKeyBytes, err := base64.StdEncoding.DecodeString(em.EncKey)
	if err != nil {
		http.Error(w, "invalid enc_key", http.StatusBadRequest)
		return
	}
	nonce, err := base64.StdEncoding.DecodeString(em.Nonce)
	if err != nil {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}
	ct, err := base64.StdEncoding.DecodeString(em.Ciphertext)
	if err != nil {
		http.Error(w, "invalid ciphertext", http.StatusBadRequest)
		return
	}

	// Load server private key to decrypt AES key
	serverPriv, err := loadRSAPrivateKeyFromFileServer("server.key")
	if err != nil {
		http.Error(w, "server private key error", http.StatusInternalServerError)
		return
	}
	// Use RSA-OAEP with SHA-256
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, serverPriv, encKeyBytes, nil)
	if err != nil {
		http.Error(w, "failed to decrypt AES key", http.StatusInternalServerError)
		return
	}

	// Decrypt payload
	plain, err := aesGCMDecryptServer(aesKey, nonce, ct)
	if err != nil {
		http.Error(w, "aes decrypt failed", http.StatusInternalServerError)
		return
	}
	log.Printf("server: received plain: %s\n", string(plain))

	// Process (append data)
	respPlain := append([]byte(string(plain)+" + server-data"), byte(0)) // small example; we append text

	// Now encrypt response for the client using client's public key (from TLS peer cert)
	clientCert := r.TLS.PeerCertificates[0]
	clientPub, ok := clientCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		http.Error(w, "client public key not RSA", http.StatusInternalServerError)
		return
	}
	// create new AES key for response
	respAESKey := make([]byte, 32) // AES-256
	if _, err := rand.Read(respAESKey); err != nil {
		http.Error(w, "failed to generate aes key", http.StatusInternalServerError)
		return
	}
	// encrypt resp payload
	respCT, respNonce, err := aesGCMEncryptServer(respAESKey, respPlain)
	if err != nil {
		http.Error(w, "aes encrypt failed", http.StatusInternalServerError)
		return
	}
	// encrypt respAESKey with client's RSA pub (OAEP SHA256)
	encRespKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, clientPub, respAESKey, nil)
	if err != nil {
		http.Error(w, "rsa encrypt failed", http.StatusInternalServerError)
		return
	}
	out := EncryptedMessage{
		EncKey:     base64.StdEncoding.EncodeToString(encRespKey),
		Nonce:      base64.StdEncoding.EncodeToString(respNonce),
		Ciphertext: base64.StdEncoding.EncodeToString(respCT),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func Security_v00_Server_Main() {
	// initial load
	cfg, err := loadServerTLSConfig()
	if err != nil {
		log.Fatalf("initial load failed: %v", err)
	}
	// store initial config
	tlsConfigAtomic.Store(cfg)

	// Setup a "wrapper" tls.Config with GetConfigForClient that returns the latest config
	wrapper := &tls.Config{
		GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
			cfg := tlsConfigAtomic.Load().(*tls.Config)
			return cfg, nil
		},
		MinVersion: tls.VersionTLS12,
	}

	// Start watching cert files
	go watchCerts()

	// HTTP handlers
	http.HandleFunc("/api/process", processHandler)
	srv := &http.Server{
		Addr:      ":8443",
		TLSConfig: wrapper,
		// good practice timeouts
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logHG.DebugInfo("HTTPS server (mTLS) listening on https://localhost:8443")
	// Note: we pass empty certFile/keyFile to ListenAndServeTLS because TLS is handled by wrapper.GetConfigForClient
	if err := srv.ListenAndServeTLS("", ""); err != nil {
		logHG.ErrFInfo("server failed: %v", err)
	}
}
