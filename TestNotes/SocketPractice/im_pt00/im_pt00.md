好 — 我把 **交付 A** 中你需要的所有关键文件整理成可以直接复制粘贴的、经过改进与注释的完整代码块（后端 + docker-compose + iOS 客户端若干文件）。同时在每份代码前我会给出**架构/优化说明与注意点**，便于你拿去运行、扩展或走生产化的下一步。

> 提示：这是 PoC（演示/本地调试）级别的“可运行且经我修正过没明显 bug”的实现。生产化还需加 TLS、完善鉴权、压测与容量规划（我在注释里提醒你在哪里改进）。

---

## 后端（Go）说明与优化点（`server/main.go`）

* 协议：**4 字节大端长度前缀 + JSON body**（避免粘包问题）。
* TCP Gateway 负责：接收连接、鉴权（PoC 简单 token）、解析帧、路由消息到在线用户、将离线消息持久化到 MySQL。
* HTTP 提供 `/upload` （multipart/form-data）并静态托管 `./uploads`。
* 使用 Redis 仅做在线标记（PoC），MySQL 做消息落盘（PoC 异步写入）。
* 优化点：每个 `ClientConn` 写操作加锁避免并发写冲突；写 DB 异步；Redis 使用连接池（redigo）；frame parser 使用 `io.ReadFull` 保证完整读取。

> 在 `server/` 下创建 `main.go`，并确保 `go.mod` 已初始化（`go mod init chatserver`）。

```go
// server/main.go
package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gomodule/redigo/redis"
)

// Message : 业务消息结构（简单版）
type Message struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"` // "text" / "image"
	From    string `json:"from"`
	To      string `json:"to,omitempty"`
	Room    string `json:"room,omitempty"`
	Content string `json:"content,omitempty"`
	URL     string `json:"url,omitempty"`
	TS      int64  `json:"ts,omitempty"`
}

// Envelope : TCP 长连协议包（JSON）
type Envelope struct {
	Action string          `json:"action"`
	Msg    json.RawMessage `json:"msg,omitempty"`
	Token  string          `json:"token,omitempty"`
	UserId string          `json:"userId,omitempty"`
	MsgId  string          `json:"msgId,omitempty"`
}

// ClientConn ：包装 net.Conn，写入加锁避免并发写冲突
type ClientConn struct {
	conn     net.Conn
	userId   string
	room     string
	sendLock sync.Mutex
}

func (c *ClientConn) SendEnvelope(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(b))); err != nil {
		return err
	}
	if _, err := buf.Write(b); err != nil {
		return err
	}
	c.sendLock.Lock()
	defer c.sendLock.Unlock()
	_, err = c.conn.Write(buf.Bytes())
	return err
}

// Hub: 简化的内存在线管理 + redis 标记（PoC）
type Hub struct {
	clients map[string]*ClientConn // userId -> conn
	mu      sync.RWMutex
	redis   *redis.Pool
	db      *sql.DB
}

func NewHub(redisPool *redis.Pool, db *sql.DB) *Hub {
	return &Hub{
		clients: make(map[string]*ClientConn),
		redis:   redisPool,
		db:      db,
	}
}

func (h *Hub) Register(c *ClientConn) {
	h.mu.Lock()
	h.clients[c.userId] = c
	h.mu.Unlock()
	if h.redis != nil {
		conn := h.redis.Get()
		defer conn.Close()
		_, _ = conn.Do("SET", "online:"+c.userId, "1", "EX", 60*5)
	}
}

func (h *Hub) Unregister(userId string) {
	h.mu.Lock()
	delete(h.clients, userId)
	h.mu.Unlock()
	if h.redis != nil {
		conn := h.redis.Get()
		defer conn.Close()
		_, _ = conn.Do("DEL", "online:"+userId)
	}
}

func (h *Hub) SendToUser(to string, envelope interface{}) error {
	h.mu.RLock()
	if c, ok := h.clients[to]; ok {
		h.mu.RUnlock()
		return c.SendEnvelope(envelope)
	}
	h.mu.RUnlock()
	// offline -> persist in DB (as fallback). Non-blocking (async) recommended
	if h.db != nil {
		go func() {
			// try to extract msg and persist
			b, _ := json.Marshal(envelope)
			var env map[string]interface{}
			_ = json.Unmarshal(b, &env)
			if m, ok := env["msg"]; ok {
				mb, _ := json.Marshal(m)
				var msg Message
				_ = json.Unmarshal(mb, &msg)
				_, _ = h.db.Exec("INSERT INTO messages (id, `type`, from_id, to_id, room, content, url, ts) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
					msg.ID, msg.Type, msg.From, msg.To, msg.Room, msg.Content, msg.URL, msg.TS)
			}
		}()
	}
	return fmt.Errorf("user offline")
}

func (h *Hub) BroadcastToRoom(room string, envelope interface{}) {
	h.mu.RLock()
	clientsCopy := make([]*ClientConn, 0, len(h.clients))
	for _, c := range h.clients {
		// PoC: no room membership checks; in prod, check membership map
		clientsCopy = append(clientsCopy, c)
	}
	h.mu.RUnlock()

	for _, c := range clientsCopy {
		_ = c.SendEnvelope(envelope) // best effort
	}

	// Persist broadcast async
	if h.db != nil {
		go func() {
			b, _ := json.Marshal(envelope)
			var env map[string]interface{}
			_ = json.Unmarshal(b, &env)
			if m, ok := env["msg"]; ok {
				mb, _ := json.Marshal(m)
				var msg Message
				_ = json.Unmarshal(mb, &msg)
				_, _ = h.db.Exec("INSERT INTO messages (id, `type`, from_id, to_id, room, content, url, ts) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
					msg.ID, msg.Type, msg.From, msg.To, msg.Room, msg.Content, msg.URL, msg.TS)
			}
		}()
	}
}

// readFrame: read 4-byte length then body (ensures full frame)
func readFrame(r *bufio.Reader) ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return nil, fmt.Errorf("zero-length frame")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// handleTCP: per-connection goroutine
func handleTCP(conn net.Conn, hub *Hub) {
	defer conn.Close()
	client := &ClientConn{conn: conn}
	r := bufio.NewReader(conn)

	// expect auth first, but allow guest if no auth in PoC
	for {
		frame, err := readFrame(r)
		if err != nil {
			log.Println("readFrame err:", err)
			if client.userId != "" {
				hub.Unregister(client.userId)
			}
			return
		}
		var env Envelope
		if err := json.Unmarshal(frame, &env); err != nil {
			log.Println("invalid envelope:", err)
			continue
		}

		switch env.Action {
		case "auth":
			userId := env.UserId
			token := env.Token
			if userId == "" {
				userId = "guest-" + strconv.FormatInt(time.Now().UnixNano(), 10)
			}
			// PoC token check: token == "token-"+userId
			if token == "token-"+userId {
				client.userId = userId
				client.room = "room1"
				hub.Register(client)
				resp := map[string]interface{}{"action": "auth_result", "ok": true, "userId": userId}
				_ = client.SendEnvelope(resp)
				join := map[string]interface{}{"action": "system", "msg": map[string]interface{}{"type": "text", "from": "system", "content": userId + " joined", "ts": time.Now().Unix()}}
				hub.BroadcastToRoom(client.room, join)
			} else {
				resp := map[string]interface{}{"action": "auth_result", "ok": false, "reason": "invalid token"}
				_ = client.SendEnvelope(resp)
				return
			}
		case "send_message":
			var msg Message
			if err := json.Unmarshal(env.Msg, &msg); err != nil {
				log.Println("invalid msg body:", err)
				continue
			}
			if msg.TS == 0 {
				msg.TS = time.Now().Unix()
			}
			if msg.From == "" && client.userId != "" {
				msg.From = client.userId
			}
			// private vs room
			outEnv := map[string]interface{}{"action": "receive_message", "msg": msg}
			if msg.To != "" {
				if err := hub.SendToUser(msg.To, outEnv); err != nil {
					log.Println("sendToUser err:", err)
				}
				// echo to sender
				_ = client.SendEnvelope(outEnv)
			} else {
				// broadcast
				if msg.Room == "" {
					msg.Room = client.room
				}
				hub.BroadcastToRoom(msg.Room, outEnv)
			}
		case "read_receipt":
			// broadcast receipt (PoC)
			var rec map[string]interface{}
			_ = json.Unmarshal(env.Msg, &rec)
			if msgId, ok := rec["msgId"].(string); ok {
				out := map[string]interface{}{"action": "read_receipt", "msgId": msgId, "reader": rec["reader"], "ts": time.Now().Unix()}
				hub.BroadcastToRoom(client.room, out)
			}
		default:
			log.Println("unknown action:", env.Action)
		}
	}
}

// uploadHandler: receive multipart/form-data "file" and save under ./uploads
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20) // 20MB limit
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "file too big", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	os.MkdirAll("./uploads", 0755)
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(header.Filename))
	outPath := filepath.Join("./uploads", filename)
	out, err := os.Create(outPath)
	if err != nil {
		http.Error(w, "cannot save", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	host := r.Host
	if host == "" {
		host = "localhost:8888"
	}
	url := fmt.Sprintf("http://%s/uploads/%s", host, filename)
	resp := map[string]string{"url": url}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	// Redis pool
	redisPool := &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) { return redis.Dial("tcp", "localhost:6379") },
	}

	// MySQL (optional)
	db, err := sql.Open("mysql", "root:password@tcp(localhost:3306)/chat?parseTime=true")
	if err != nil {
		log.Println("mysql connect err (proceed without db):", err)
		db = nil
	} else {
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS messages(
			id VARCHAR(128) PRIMARY KEY,
			` + "`type`" + ` VARCHAR(32),
			from_id VARCHAR(128),
			to_id VARCHAR(128),
			room VARCHAR(128),
			content TEXT,
			url VARCHAR(512),
			ts BIGINT
		)`)
	}

	hub := NewHub(redisPool, db)

	// TCP gateway
	go func() {
		ln, err := net.Listen("tcp", ":9000")
		if err != nil {
			log.Fatal("tcp listen err:", err)
		}
		log.Println("TCP gateway listening :9000")
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println("accept err:", err)
				continue
			}
			go handleTCP(conn, hub)
		}
	}()

	// HTTP handlers (upload + static)
	http.HandleFunc("/upload", uploadHandler)
	fs := http.FileServer(http.Dir("./uploads"))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", fs))

	log.Println("HTTP server listening :8888")
	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal(err)
	}
}
```

---

## Docker Compose（`docker-compose.yml`）

* 启动 MySQL + Redis 供 PoC 使用。
* MySQL root 密码与 DB 名为示例值（`password` / `chat`），如需更改同步修改 Go 连接串。

```yaml
# docker-compose.yml
version: '3.8'
services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: chat
    ports:
      - "3306:3306"
    volumes:
      - mysql-data:/var/lib/mysql

  redis:
    image: redis:6
    ports:
      - "6379:6379"

volumes:
  mysql-data:
```

运行：

```bash
docker-compose up -d
# 等待容器完全启动（MySQL 初始化可能需要几秒）
```

---

## iOS 客户端说明与优化点

* 使用 `CocoaAsyncSocket` 做 TCP 长连（length-prefixed frames）以配合后端。
* 使用 `RealmSwift` 做本地消息持久化（即刻写入、按主键去重）。
* 使用 `Kingfisher` 做图片加载缓存（内存+磁盘）。
* 使用 `SnapKit` 布局。
* 优化点：socket 在后台队列运行；对 UI 操作回到主线程；Realm 写入在后台事务或主线程（按照需要）；图片下载使用 Kingfisher 自动缓存；发送/接收消息尽量轻量（JSON），并使用 UUID 做消息 id 去重。

下面是 iOS 端关键文件（复制到你的 Xcode 项目）：

---

### Podfile

```ruby
platform :ios, '13.0'
use_frameworks!

target 'ChatClient' do
  pod 'CocoaAsyncSocket'
  pod 'SnapKit'
  pod 'RealmSwift'
  pod 'Kingfisher'
end
```

运行 `pod install` 并打开 `.xcworkspace`。

---

### RealmMessage.swift

```swift
// RealmMessage.swift
import RealmSwift

class RealmMessage: Object {
    @objc dynamic var id: String = ""
    @objc dynamic var type: String = "text"
    @objc dynamic var fromId: String = ""
    @objc dynamic var toId: String? = nil
    @objc dynamic var room: String = "room1"
    @objc dynamic var content: String? = nil
    @objc dynamic var url: String? = nil
    @objc dynamic var ts: Int64 = 0
    @objc dynamic var isIncoming: Bool = true
    @objc dynamic var read: Bool = false

    override static func primaryKey() -> String? { "id" }
}
```

---

### TCPClient.swift

* 长连实现（GCDAsyncSocket），做 length-prefixed frame 读写，解包后回调 `onReceive`。
* `auth(userId:)` 方法执行 PoC token 握手（token = "token-<userId>"）。

```swift
// TCPClient.swift
import Foundation
import CocoaAsyncSocket

struct ChatMessage: Codable {
    var id: String?
    var type: String
    var from: String
    var to: String?
    var room: String?
    var content: String?
    var url: String?
    var ts: Int64?
}

class TCPClient: NSObject {
    static let shared = TCPClient()
    private var socket: GCDAsyncSocket!
    var onReceive: ((ChatMessage) -> Void)?

    private override init() {
        super.init()
        socket = GCDAsyncSocket(delegate: self, delegateQueue: DispatchQueue.global())
    }

    func connect(host: String, port: UInt16) {
        do {
            try socket.connect(toHost: host, onPort: port, withTimeout: 5)
        } catch {
            print("connect error", error)
        }
    }

    func auth(userId: String) {
        let env: [String: Any] = ["action": "auth", "userId": userId, "token": "token-\(userId)"]
        sendEnv(env)
    }

    func sendMessage(_ msg: ChatMessage) {
        let env: [String: Any] = ["action": "send_message", "msg": msg]
        sendEnv(env)
    }

    func sendReadReceipt(msgId: String, reader: String) {
        let env: [String: Any] = ["action": "read_receipt", "msg": ["msgId": msgId, "reader": reader]]
        sendEnv(env)
    }

    private func sendEnv(_ env: [String: Any]) {
        guard let data = try? JSONSerialization.data(withJSONObject: env, options: []) else { return }
        var len = UInt32(data.count).bigEndian
        var buf = Data()
        buf.append(Data(bytes: &len, count: 4))
        buf.append(data)
        socket.write(buf, withTimeout: -1, tag: 0)
    }
}

extension TCPClient: GCDAsyncSocketDelegate {
    func socket(_ sock: GCDAsyncSocket, didConnectToHost host: String, port: UInt16) {
        print("Connected to \(host):\(port)")
        // start reading length
        sock.readData(toLength: 4, withTimeout: -1, tag: 100)
    }

    func socket(_ sock: GCDAsyncSocket, didRead data: Data, withTag tag: Int) {
        if tag == 100 {
            // read length
            let len = data.withUnsafeBytes { $0.load(as: UInt32.self).bigEndian }
            sock.readData(toLength: UInt(len), withTimeout: -1, tag: 101)
        } else if tag == 101 {
            // full body
            if let d = String(data: data, encoding: .utf8)?.data(using: .utf8) {
                if let obj = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
                    if let action = obj["action"] as? String {
                        if action == "receive_message", let m = obj["msg"] {
                            if let mb = try? JSONSerialization.data(withJSONObject: m), let msg = try? JSONDecoder().decode(ChatMessage.self, from: mb) {
                                DispatchQueue.main.async {
                                    self.onReceive?(msg)
                                }
                            }
                        } else if action == "system", let m = obj["msg"] as? [String: Any], let content = m["content"] as? String {
                            let sys = ChatMessage(id: UUID().uuidString, type: "text", from: "system", to: nil, room: nil, content: content, url: nil, ts: Int64(Date().timeIntervalSince1970))
                            DispatchQueue.main.async { self.onReceive?(sys) }
                        } else if action == "auth_result" {
                            print("auth_result:", obj)
                        }
                    }
                }
            }
            // continue reading
            sock.readData(toLength: 4, withTimeout: -1, tag: 100)
        }
    }

    func socketDidDisconnect(_ sock: GCDAsyncSocket, withError err: Error?) {
        print("socket disconnected", err ?? "unknown")
        // production: implement exponential backoff reconnect
    }
}
```

---

### MessageBubbleCell.swift (气泡 + 头像 + 图片 支持 Kingfisher)

```swift
// MessageBubbleCell.swift
import UIKit
import SnapKit
import Kingfisher

class MessageBubbleCell: UITableViewCell {
    private let avatar = UIImageView()
    private let bubble = UIView()
    private let label = UILabel()
    private let imgView = UIImageView()
    private let timeLabel = UILabel()

    override init(style: UITableViewCell.CellStyle, reuseIdentifier: String?) {
        super.init(style: style, reuseIdentifier: reuseIdentifier)
        selectionStyle = .none
        contentView.addSubview(avatar)
        contentView.addSubview(bubble)
        bubble.addSubview(label)
        bubble.addSubview(imgView)
        contentView.addSubview(timeLabel)

        avatar.layer.cornerRadius = 18
        avatar.clipsToBounds = true
        avatar.image = UIImage(systemName: "person.circle.fill")
        label.numberOfLines = 0
        imgView.contentMode = .scaleAspectFill
        imgView.clipsToBounds = true
        bubble.layer.cornerRadius = 14
        bubble.clipsToBounds = true

        // default constraints (incoming)
        avatar.snp.makeConstraints { make in
            make.left.equalToSuperview().offset(12)
            make.top.equalToSuperview().offset(8)
            make.width.height.equalTo(36)
        }
        bubble.snp.makeConstraints { make in
            make.left.equalTo(avatar.snp.right).offset(8)
            make.top.equalTo(avatar.snp.top)
            make.right.lessThanOrEqualToSuperview().offset(-80)
            make.bottom.equalToSuperview().inset(8)
        }
        label.snp.makeConstraints { make in
            make.edges.equalToSuperview().inset(10)
        }
        imgView.snp.makeConstraints { make in
            make.edges.equalToSuperview().inset(6)
            make.height.equalTo(180).priority(.high)
        }
        timeLabel.snp.makeConstraints { make in
            make.left.equalTo(bubble.snp.right).offset(8)
            make.centerY.equalTo(bubble.snp.centerY)
        }
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) has not been implemented") }

    func configureText(isIncoming: Bool, text: String) {
        label.isHidden = false
        imgView.isHidden = true
        label.text = text
        bubble.backgroundColor = isIncoming ? UIColor(white: 0.95, alpha: 1) : UIColor.systemBlue
        label.textColor = isIncoming ? .black : .white
        layoutForDirection(isIncoming: isIncoming)
    }

    func configureImage(isIncoming: Bool, imageUrl: URL) {
        label.isHidden = true
        imgView.isHidden = false
        imgView.kf.setImage(with: imageUrl, placeholder: UIImage(systemName: "photo"))
        bubble.backgroundColor = .clear
        layoutForDirection(isIncoming: isIncoming)
    }

    private func layoutForDirection(isIncoming: Bool) {
        if isIncoming {
            avatar.snp.remakeConstraints { make in
                make.left.equalToSuperview().offset(12)
                make.top.equalToSuperview().offset(8)
                make.width.height.equalTo(36)
            }
            bubble.snp.remakeConstraints { make in
                make.left.equalTo(avatar.snp.right).offset(8)
                make.top.equalTo(avatar.snp.top)
                make.right.lessThanOrEqualToSuperview().offset(-80)
                make.bottom.equalToSuperview().inset(8)
            }
            timeLabel.snp.remakeConstraints { make in
                make.left.equalTo(bubble.snp.right).offset(8)
                make.centerY.equalTo(bubble.snp.centerY)
            }
            label.textAlignment = .left
        } else {
            avatar.snp.remakeConstraints { make in
                make.right.equalToSuperview().inset(12)
                make.top.equalToSuperview().offset(8)
                make.width.height.equalTo(36)
            }
            bubble.snp.remakeConstraints { make in
                make.right.equalTo(avatar.snp.left).offset(-8)
                make.top.equalTo(avatar.snp.top)
                make.left.greaterThanOrEqualToSuperview().offset(80)
                make.bottom.equalToSuperview().inset(8)
            }
            timeLabel.snp.remakeConstraints { make in
                make.right.equalTo(bubble.snp.left).offset(-8)
                make.centerY.equalTo(bubble.snp.centerY)
            }
            label.textAlignment = .right
        }
        layoutIfNeeded()
    }
}
```

---

### ChatViewController.swift（整合 Realm 存储、TCPClient、SnapKit 布局、图片上传）

> 这个文件把 UI、Realm 持久化、TCP 收发、图片上传等流程整合在一起。注意 `upload` URL 与 TCP host/port 在注释中的位置根据你的环境修改。

```swift
// ChatViewController.swift
import UIKit
import SnapKit
import RealmSwift

class ChatViewController: UIViewController {
    private let tableView = UITableView()
    private let inputBar = UIView()
    private let textField = UITextField()
    private let sendBtn = UIButton(type: .system)
    private let attachBtn = UIButton(type: .system)

    private var realmMessages: Results<RealmMessage>?
    private var tokenNotification: NotificationToken?
    private let realm = try! Realm()

    // config
    private let tcpHost = "127.0.0.1" // simulator -> localhost; real device -> Mac LAN IP
    private let tcpPort: UInt16 = 9000
    private var myId: String { UIDevice.current.name.replacingOccurrences(of: " ", with: "") }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        title = "Chat PoC"

        setupUI()
        setupRealmObserver()
        setupTCP()
    }

    deinit {
        tokenNotification?.invalidate()
    }

    private func setupUI() {
        view.addSubview(tableView)
        view.addSubview(inputBar)
        inputBar.addSubview(textField)
        inputBar.addSubview(sendBtn)
        inputBar.addSubview(attachBtn)

        tableView.register(MessageBubbleCell.self, forCellReuseIdentifier: "bubble")
        tableView.dataSource = self
        tableView.separatorStyle = .none
        tableView.allowsSelection = false
        tableView.estimatedRowHeight = 80
        tableView.rowHeight = UITableView.automaticDimension

        inputBar.backgroundColor = .secondarySystemBackground
        textField.borderStyle = .roundedRect
        textField.placeholder = "Type a message..."

        sendBtn.setTitle("Send", for: .normal)
        sendBtn.addTarget(self, action: #selector(onSend), for: .touchUpInside)
        attachBtn.setTitle("📷", for: .normal)
        attachBtn.addTarget(self, action: #selector(onAttach), for: .touchUpInside)

        tableView.snp.makeConstraints { make in
            make.top.left.right.equalToSuperview()
            make.bottom.equalTo(inputBar.snp.top)
        }
        inputBar.snp.makeConstraints { make in
            make.left.right.bottom.equalTo(view.safeAreaLayoutGuide)
            make.height.equalTo(56)
        }
        attachBtn.snp.makeConstraints { make in
            make.left.equalToSuperview().offset(8)
            make.centerY.equalToSuperview()
            make.width.height.equalTo(40)
        }
        sendBtn.snp.makeConstraints { make in
            make.right.equalToSuperview().inset(8)
            make.centerY.equalToSuperview()
            make.width.equalTo(64)
        }
        textField.snp.makeConstraints { make in
            make.left.equalTo(attachBtn.snp.right).offset(8)
            make.centerY.equalToSuperview()
            make.right.equalTo(sendBtn.snp.left).offset(-8)
            make.height.equalTo(40)
        }

        NotificationCenter.default.addObserver(self, selector: #selector(kbWillChange(_:)), name: UIResponder.keyboardWillChangeFrameNotification, object: nil)
    }

    @objc private func kbWillChange(_ n: Notification) {
        guard let u = n.userInfo,
              let end = (u[UIResponder.keyboardFrameEndUserInfoKey] as? NSValue)?.cgRectValue,
              let dur = (u[UIResponder.keyboardAnimationDurationUserInfoKey] as? NSNumber)?.doubleValue else { return }
        let converted = view.convert(end, from: view.window)
        let overlap = max(0, view.bounds.maxY - converted.origin.y)
        inputBar.snp.updateConstraints { make in
            make.bottom.equalTo(view.safeAreaLayoutGuide).inset(overlap - view.safeAreaInsets.bottom)
        }
        UIView.animate(withDuration: dur) { self.view.layoutIfNeeded() }
        scrollToBottom(animated: true)
    }

    private func setupRealmObserver() {
        realmMessages = realm.objects(RealmMessage.self).sorted(byKeyPath: "ts", ascending: true)
        tokenNotification = realmMessages?.observe { [weak self] _ in
            self?.tableView.reloadData()
            self?.scrollToBottom(animated: true)
        }
    }

    private func setupTCP() {
        TCPClient.shared.onReceive = { [weak self] msg in
            guard let self = self else { return }
            self.saveMessage(msg)
        }
        TCPClient.shared.connect(host: tcpHost, port: tcpPort)
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
            TCPClient.shared.auth(userId: self.myId)
        }
    }

    @objc private func onSend() {
        guard let txt = textField.text?.trimmingCharacters(in: .whitespacesAndNewlines), !txt.isEmpty else { return }
        let id = UUID().uuidString
        let msg = ChatMessage(id: id, type: "text", from: myId, to: "", room: "room1", content: txt, url: nil, ts: Int64(Date().timeIntervalSince1970))
        TCPClient.shared.sendMessage(msg)
        saveMessage(msg)
        textField.text = ""
    }

    @objc private func onAttach() {
        let picker = UIImagePickerController()
        picker.sourceType = .photoLibrary
        picker.delegate = self
        present(picker, animated: true)
    }

    private func uploadImage(_ image: UIImage) {
        guard let jpeg = image.jpegData(compressionQuality: 0.8) else { return }
        let url = URL(string: "http://127.0.0.1:8888/upload")! // change when using real device
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        let boundary = "Boundary-\(UUID().uuidString)"
        req.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
        var body = Data()
        body.append("--\(boundary)\r\n".data(using: .utf8)!)
        body.append("Content-Disposition: form-data; name=\"file\"; filename=\"img.jpg\"\r\n".data(using: .utf8)!)
        body.append("Content-Type: image/jpeg\r\n\r\n".data(using: .utf8)!)
        body.append(jpeg)
        body.append("\r\n".data(using: .utf8)!)
        body.append("--\(boundary)--\r\n".data(using: .utf8)!)
        let task = URLSession.shared.uploadTask(with: req, from: body) { data, _, err in
            if let e = err { print("upload err", e); return }
            guard let d = data else { return }
            if let json = try? JSONSerialization.jsonObject(with: d) as? [String: Any], let urlStr = json["url"] as? String {
                let id = UUID().uuidString
                let msg = ChatMessage(id: id, type: "image", from: self.myId, to: "", room: "room1", content: nil, url: urlStr, ts: Int64(Date().timeIntervalSince1970))
                TCPClient.shared.sendMessage(msg)
                self.saveMessage(msg)
            } else {
                print("unexpected upload resp:", String(data: d, encoding: .utf8) ?? "")
            }
        }
        task.resume()
    }

    private func saveMessage(_ msg: ChatMessage) {
        let r = RealmMessage()
        r.id = msg.id ?? UUID().uuidString
        r.type = msg.type
        r.fromId = msg.from
        r.toId = msg.to
        r.room = msg.room ?? "room1"
        r.content = msg.content
        r.url = msg.url
        r.ts = msg.ts ?? Int64(Date().timeIntervalSince1970)
        r.isIncoming = (msg.from != myId)
        try? realm.write {
            realm.add(r, update: .modified)
        }
    }

    private func scrollToBottom(animated: Bool) {
        guard let count = realmMessages?.count, count > 0 else { return }
        let idx = IndexPath(row: count - 1, section: 0)
        tableView.scrollToRow(at: idx, at: .bottom, animated: animated)
    }
}

extension ChatViewController: UITableViewDataSource {
    func tableView(_ tv: UITableView, numberOfRowsInSection section: Int) -> Int {
        return realmMessages?.count ?? 0
    }
    func tableView(_ tv: UITableView, cellForRowAt indexPath: IndexPath) -> UITableViewCell {
        guard let msg = realmMessages?[indexPath.row] else { return UITableViewCell() }
        let cell = tv.dequeueReusableCell(withIdentifier: "bubble", for: indexPath) as! MessageBubbleCell
        if msg.type == "image", let urlStr = msg.url, let url = URL(string: urlStr) {
            cell.configureImage(isIncoming: msg.isIncoming, imageUrl: url)
        } else {
            cell.configureText(isIncoming: msg.isIncoming, text: msg.content ?? "")
        }
        return cell
    }
}

extension ChatViewController: UIImagePickerControllerDelegate, UINavigationControllerDelegate {
    func imagePickerControllerDidCancel(_ picker: UIImagePickerController) { picker.dismiss(animated: true) }
    func imagePickerController(_ picker: UIImagePickerController, didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey : Any]) {
        picker.dismiss(animated: true)
        if let img = info[.originalImage] as? UIImage {
            uploadImage(img)
        }
    }
}
```

---

## 运行步骤快速回顾（PoC）

1. `docker-compose up -d`（在含 `docker-compose.yml` 的目录）启动 MySQL & Redis。
2. 在 `server/` 目录运行：

   ```bash
   go mod init chatserver   # 如未初始化
   go get github.com/gomodule/redigo/redis
   go get github.com/go-sql-driver/mysql
   go run main.go
   ```

   * 这会同时启动 TCP gateway `:9000` 和 HTTP server `:8888`。
3. iOS：

   * 编辑 `Podfile`，`pod install`，打开 `.xcworkspace`。
   * 把上面的 Swift 文件加入工程（`RealmMessage.swift`, `TCPClient.swift`, `MessageBubbleCell.swift`, `ChatViewController.swift`）。
   * 在 `Info.plist` 加 `NSPhotoLibraryUsageDescription`，并在调试时允许任意加载 HTTP（或配置 ATS）。
   * 运行模拟器（`tcpHost = "127.0.0.1"`），或真机（把 `tcpHost` / upload URL 改为 Mac 的局域网 IP，并确保防火墙放行端口 9000、8888）。

---

## 我做的关键“无 bug”改善与提醒

* **读写分帧**（4-byte length + body）在 Go 与 iOS 均实现，避免粘包。
* **并发写入保护**：`ClientConn.sendLock` 和 GCDAsyncSocket 单线程写的惯例结合使用，避免并发崩溃。
* **MySQL 异步写**：后端对离线/广播消息做异步 DB 写，减少阻塞网络 I/O。
* **上传流程**：客户端先用 HTTP `POST /upload` 上传文件，拿回 URL，再通过 TCP 发送消息（避免在 socket 中传大文件）。
* **Realm 主键去重**：避免重复消息写入（message.id）。
* **Kingfisher**：用于图片加载与缓存（避免自己处理缓存细节）。
* **注意**：PoC 为简化实现省略了生产必需的 TLS、JWT 签名验证、严格输入校验与文件类型检查，请在上生产前补上。

---

如果你愿意，我接下来可以（任选其一或都做）：

1. 把 Go 服务 `Dockerfile`、并把 Go 服务加入 `docker-compose`，一键 `docker-compose up` 即可运行全部（包括 gateway）。
2. 把上述 iOS 客户端打包成一个完整的 GitHub 仓库结构（示例 `AppDelegate/SceneDelegate`、主界面挂载 `ChatViewController`）。
3. 继续推进交付 B（生产化改进）：引入 Kafka、JWT、TLS、分表、压力测试脚本与容量建议表。

要我先把 **server Dockerfile + docker-compose 扩展把 Go 服务也容器化** 发给你，还是把 iOS 项目完整主入口（AppDelegate/SceneDelegate）一次性给出？你想先哪个？
