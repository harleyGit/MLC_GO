package HGSafeV0Pkg

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// -------------------- User store --------------------
// 简单内存用户库（生产用DB）
type UserStore struct {
	mu    sync.Mutex
	users map[string][]byte // username -> bcrypt hash
}

func NewUserStore() *UserStore { return &UserStore{users: map[string][]byte{}} }
func (s *UserStore) Save(username string, bcryptHash []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[username] = bcryptHash
}
func (s *UserStore) Exists(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.users[username]
	return ok
}

// -------------------- Session management --------------------

type Session struct {
	Priv   [32]byte
	Pub    [32]byte
	Expiry time.Time
	Used   bool
}

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore { return &SessionStore{sessions: map[string]*Session{}} }

func (s *SessionStore) Put(id string, sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = sess
}

// GetAndConsume: get session only if exists, not expired, not used; consume it (delete)
func (s *SessionStore) GetAndConsume(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	// 如果过期或已用，删除并返回 false 
	// Expiry 示例是 2 分钟，CleanupLoop 每 30 秒清理一次过期 session
	if time.Now().After(sess.Expiry) || sess.Used {
		delete(s.sessions, id)
		return nil, false
	}
	// 标记为已用并返回（保证 single-use）
	sess.Used = true
	delete(s.sessions, id)// 删除以保证不可再次使用
	return sess, true
}

func (s *SessionStore) CleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for id, sess := range s.sessions {
			if now.After(sess.Expiry) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

// -------------------- helpers --------------------

func randSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	// 使用 RawStdEncoding 产生无等号的 base64 id
	return base64.RawStdEncoding.EncodeToString(b), nil
}

func newEphemeralKeypair() ([32]byte, [32]byte, error) {
	var priv, pub [32]byte
	// 生成32字节随机密钥（x25519 私钥的 clamping 在 curve25519.x25519做）
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return priv, pub, err
	}
	//计算公钥
	p, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return priv, pub, err
	}
	copy(pub[:], p)
	return priv, pub, nil
}

// -------------------- global instances --------------------

var (
	store        = NewUserStore()
	sessionStore = NewSessionStore()
)

// -------------------- HTTP handlers --------------------
// StartSessionResponse 返回 session_id 与 server_pub (base64 raw)
type StartSessionResp struct {
	SessionID string `json:"session_id"`
	ServerPub string `json:"server_pub"` // base64 raw
}

// POST /start_session
// 返回 ephemeral server pub 和 session id（有效期短）
func startSessionHandler(w http.ResponseWriter, r *http.Request) {
	// create ephemeral keypair
	// 生成 ephemeral key
	priv, pub, err := newEphemeralKeypair()
	if err != nil {
		http.Error(w, "keygen failed", http.StatusInternalServerError)
		return
	}
	sessionID, err := randSessionID()
	if err != nil {
		http.Error(w, "session id failed", http.StatusInternalServerError)
		return
	}

	// 存入 session store，设置短期过期（例如 2 分钟）
	sess := &Session{Priv: priv, 
		Pub: pub, Expiry: time.Now().Add(2 * time.Minute), Used: false}
	sessionStore.Put(sessionID, sess)

	// EncodeToString 返回 base64编码的公钥
	resp := StartSessionResp{SessionID: sessionID, ServerPub: base64.RawStdEncoding.EncodeToString(pub[:])}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Register 接收客户端加密后的密码 + session_id + client_pub
type RegisterReq struct {
	Username   string `json:"username"`
	SessionID  string `json:"session_id"`
	ClientPub  string `json:"client_pub"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// POST /register
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	// 校验 session
	if req.SessionID == "" {
		http.Error(w, "missing session_id", http.StatusBadRequest)
		return
	}
	// get and consume session (single-use)
	sess, ok := sessionStore.GetAndConsume(req.SessionID)
	if !ok {
		http.Error(w, "invalid or expired session", http.StatusUnauthorized)
		return
	}
	// decode client pub
	clientPub, err := base64.RawStdEncoding.DecodeString(req.ClientPub)
	if err != nil || len(clientPub) != 32 {
		http.Error(w, "bad client_pub", http.StatusBadRequest)
		return
	}
	nonce, err := base64.RawStdEncoding.DecodeString(req.Nonce)
	if err != nil || len(nonce) != chacha20poly1305.NonceSize {
		http.Error(w, "bad nonce", http.StatusBadRequest)
		return
	}
	// ciphertext: accept standard base64 or raw
	ct, err := base64.StdEncoding.DecodeString(req.Ciphertext) // 支持标准 base64
	if err != nil {
		// 也尝试 raw
		ct, err = base64.RawStdEncoding.DecodeString(req.Ciphertext)
		if err != nil {
			http.Error(w, "bad ciphertext", http.StatusBadRequest)
			return
		}
	}

	// compute shared secret
	// 1.计算共享密钥 X25519(shared) = X25519(server_ephemeral_priv, client_pub)
	shared, err := curve25519.X25519(sess.Priv[:], clientPub)
	if err != nil {
		http.Error(w, "x25519 failed", http.StatusInternalServerError)
		return
	}

	// 2. 使用 HKDF-SHA256 从 shared派生 32 字节 AEAD key
	// 可指定 salt/Info； 示例使用空salt，但生产请使用固定/协商的salt或会话标识
	// derive symmetric key via HKDF; use binding info that includes session/user context
	info := []byte("registration v1 ephemeral:username=" + req.Username + ";session=" + req.SessionID)
	symKey := make([]byte, chacha20poly1305.KeySize)
	h := hkdf.New(sha256.New, shared, nil, info)
	if _, err := io.ReadFull(h, symKey); err != nil {
		http.Error(w, "hkdf failed", http.StatusInternalServerError)
		return
	}

	// 3.用 ChaCha20-Poly1305解密
	aead, err := chacha20poly1305.New(symKey)
	if err != nil {
		http.Error(w, "aead new failed", http.StatusInternalServerError)
		return
	}

	// Use AAD to bind username+session to ciphertext (prevents mixups / replay across usernames)
	aad := []byte(req.Username + "|" + req.SessionID)
	plaintext, err := aead.Open(nil, nonce, ct, aad)
	if err != nil {
		http.Error(w, "decrypt failed", http.StatusUnauthorized)
		return
	}
	//plaintext 里应该包含密码（和可选的时间戳），示例直接是明文密码
	password := string(plaintext)

	// 处理：保存 bcrypt(password)
	if store.Exists(req.Username) {
		http.Error(w, "user exists", http.StatusConflict)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "bcrypt failed", http.StatusInternalServerError)
		return
	}
	store.Save(req.Username, hash)

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, `{"status":"ok"}`)
}

 func SafeMainPT() {
	// cleanup goroutine
	go sessionStore.CleanupLoop(30 * time.Second)

	http.HandleFunc("/start_session", startSessionHandler)
	http.HandleFunc("/register", registerHandler)

	addr := ":8080"
	fmt.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
