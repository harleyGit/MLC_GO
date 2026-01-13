/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-24 08:30:24
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:10:23
 * @FilePath: /MLC_GO/.vscode/security/security_v00/security_v00_client.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package securityv00

import (
	"MLC_GO/internal/pkg/logHG"
	"bytes"
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
	"net/http"
	"os"
)

// type EncryptedMessage struct {
// 	EncKey     string `json:"enc_key"`
// 	Nonce      string `json:"nonce"`
// 	Ciphertext string `json:"ciphertext"`
// }

// Load RSA private key from PEM (client.key)
func loadRSAPrivateKeyFromFile(path string) (*rsa.PrivateKey, error) {
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

// AES-GCM encrypt (same as in server)
func aesGCMEncrypt(aesKey, plaintext []byte) (ciphertext, nonce []byte, err error) {
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

func aesGCMDecrypt(aesKey, nonce, ciphertext []byte) ([]byte, error) {
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

// load server public key from server cert (server.pem)
func loadServerRSAPubFromCert(path string) (*rsa.PublicKey, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(bs)
	if block == nil {
		return nil, fmt.Errorf("no PEM in %s", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("server cert public key not RSA")
	}
	return pub, nil
}

func Security_v00_Client_Main() {
	// Load client cert for mTLS
	cert, err := tls.LoadX509KeyPair("client.pem", "client.key")
	if err != nil {
		logHG.ErrFInfo("load client cert: %v", err)
	}
	// Root CA (to trust server cert)
	caPEM, err := os.ReadFile("ca.pem")
	if err != nil {
		logHG.ErrFInfo("read ca.pem: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		logHG.ErrFInfo("append ca.pem failed")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}
	tr := &http.Transport{TLSClientConfig: tlsCfg}
	client := &http.Client{Transport: tr}

	// Prepare plaintext and encryption
	plain := []byte("hello-from-client")

	// generate AES key (32 bytes -> AES-256)
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		logHG.ErrFInfo("rand aes key: %v", err)
	}
	ct, nonce, err := aesGCMEncrypt(aesKey, plain)
	if err != nil {
		logHG.ErrFInfo("aes encrypt: %v", err)
	}

	// get server public key to encrypt AES key
	serverPub, err := loadServerRSAPubFromCert("server.pem")
	if err != nil {
		logHG.ErrFInfo("load server pub: %v", err)
	}
	encKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, serverPub, aesKey, nil)
	if err != nil {
		logHG.ErrFInfo("rsa encrypt aes key: %v", err)
	}

	msg := EncryptedMessage{
		EncKey:     base64.StdEncoding.EncodeToString(encKey),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
	}
	bs, _ := json.Marshal(msg)

	resp, err := client.Post("https://localhost:8443/api/process", "application/json", bytes.NewReader(bs))
	if err != nil {
		logHG.ErrFInfo("post error: %v", err)
	}
	defer resp.Body.Close()

	var out EncryptedMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logHG.ErrFInfo("decode resp: %v", err)
	}

	// decrypt response: first decrypt enc_key with client private key
	clientPriv, err := loadRSAPrivateKeyFromFile("client.key")
	if err != nil {
		logHG.ErrFInfo("load client priv: %v", err)
	}
	encRespKey, _ := base64.StdEncoding.DecodeString(out.EncKey)
	respAESKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, clientPriv, encRespKey, nil)
	if err != nil {
		logHG.ErrFInfo("rsa decrypt resp key: %v", err)
	}
	respNonce, _ := base64.StdEncoding.DecodeString(out.Nonce)
	respCT, _ := base64.StdEncoding.DecodeString(out.Ciphertext)
	plainResp, err := aesGCMDecrypt(respAESKey, respNonce, respCT)
	if err != nil {
		logHG.ErrFInfo("aes decrypt resp: %v", err)
	}
	logHG.DebugFInfo("Client received plain response:", string(plainResp))
}
