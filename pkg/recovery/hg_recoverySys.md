<!--
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-09-25 20:46:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-09-25 20:46:52
 * @FilePath: /MLC_GO/pkg/recovery/hg_recoverySys.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
你问的就是：**“panic 崩溃后，NSQ 怎么保证服务还能起来继续跑？”**
这就是靠 **进程守护（process supervision）** 来做的。常见做法有三种：

---

## 1. systemd（Linux 系统常见）

Linux 里现在大多数服务都用 systemd 来管理。
你可以写一个 unit 文件，例如 `/etc/systemd/system/nsqd.service`：

```ini
[Unit]
Description=NSQ Daemon
After=network.target

[Service]
ExecStart=/usr/local/bin/nsqd --data-path=/var/lib/nsq
Restart=always
RestartSec=3
User=nsq

[Install]
WantedBy=multi-user.target
```

关键点：

* `Restart=always`：进程崩溃（包括 panic）时，systemd 会自动拉起。
* `RestartSec=3`：延迟 3 秒再重启，避免频繁 crash 重启。

启动和管理：

```bash
sudo systemctl daemon-reload
sudo systemctl enable nsqd
sudo systemctl start nsqd
sudo systemctl status nsqd
```

---

## 2. supervisor（跨平台工具）

如果不用 systemd，可以用 [supervisord](http://supervisord.org/)。
写一个配置文件 `/etc/supervisord.conf`：

```ini
[program:nsqd]
command=/usr/local/bin/nsqd --data-path=/var/lib/nsq
autostart=true
autorestart=true
stderr_logfile=/var/log/nsqd.err.log
stdout_logfile=/var/log/nsqd.out.log
```

启动 supervisord 后，它会自动守护 `nsqd` 进程。
如果 panic，supervisor 会重启它。

---

## 3. Docker / Kubernetes

在容器环境里，**Docker 默认会把崩溃的容器退出码传递出来**。
你可以用 Docker 自带的 restart 策略：

```bash
docker run -d --name nsqd \
  --restart=always \
  nsqio/nsq /nsqd --data-path=/var/lib/nsq
```

或者在 **Kubernetes** 里运行时，Pod 的 `restartPolicy: Always` 会保证容器挂掉就重启。

---

✅ 总结：

* **panic → 程序崩溃退出 → 守护进程发现 → 自动重启**。
* 这就是为什么 NSQ 这类服务敢用「panic」来快速失败，而不是 try-catch 式的容错。

---

要不要我给你写一个 **完整的 systemd + 日志收集方案**（包含 panic stack trace 自动进日志文件）？



好，我们来做一个 **完整的 NSQ systemd 守护 + 日志收集方案** 🚀

---

## 1. 创建 NSQ 运行用户（非必须，但推荐）

避免直接用 `root` 运行：

```bash
sudo useradd -r -s /bin/false nsq
sudo mkdir -p /var/lib/nsq
sudo chown nsq:nsq /var/lib/nsq
```

---

## 2. 创建 systemd unit 文件

新建文件：
`/etc/systemd/system/nsqd.service`

```ini
[Unit]
Description=NSQ Daemon
After=network.target

[Service]
ExecStart=/usr/local/bin/nsqd --data-path=/var/lib/nsq --lookupd-tcp-address=127.0.0.1:4160
# 自动重启策略
Restart=always
RestartSec=3

# 日志输出到 journald（systemd 默认处理）
StandardOutput=journal
StandardError=journal

# 指定运行用户
User=nsq
Group=nsq

# 设置资源限制（可选）
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

---

## 3. 启用并启动 NSQ

```bash
sudo systemctl daemon-reload
sudo systemctl enable nsqd
sudo systemctl start nsqd
```

检查状态：

```bash
sudo systemctl status nsqd
```

---

## 4. 日志收集方式

### journald（默认）

所有日志会进 journald，可以查看：

```bash
journalctl -u nsqd -f
```

如果 panic：

* 栈信息会被完整记录在 systemd 日志里。

### 单独日志文件（可选）

如果你希望单独存日志，可以改配置：

```ini
StandardOutput=append:/var/log/nsqd.log
StandardError=append:/var/log/nsqd.err
```

记得：

```bash
sudo touch /var/log/nsqd.log /var/log/nsqd.err
sudo chown nsq:nsq /var/log/nsqd*
```

---

## 5. panic 处理和分析

当 NSQ 内部出现 panic，例如：

```
panic: interface conversion: interface {} is string, not *nsq.Message
```

systemd 会捕捉到 **退出码** 和 **stack trace**，写到日志里。
因为我们设置了：

```ini
Restart=always
RestartSec=3
```

所以 systemd 会在 3 秒后自动拉起进程，保证服务可用。

---

✅ 最终效果：

* panic → NSQ 崩溃 → systemd 记录日志 + 崩溃栈 → 3 秒后自动重启。
* 我们可以用 `journalctl` 或日志文件来排查 bug。

---

要不要我帮你写一个 **演示脚本**，故意让 NSQ panic，然后演示 systemd 自动重启的全过程？



👌 那我给你写一个 **完整的演示步骤**，你可以在测试环境里试一下：

---

## 1. 写一个“假的 NSQ 程序”，里面故意 panic

新建文件 `/usr/local/bin/fake-nsqd.go`：

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("fake-nsqd started...")

	time.Sleep(2 * time.Second)

	// 故意制造 panic
	panic("fake-nsqd: something went wrong!!!")
}
```

编译：

```bash
go build -o /usr/local/bin/fake-nsqd /usr/local/bin/fake-nsqd.go
chmod +x /usr/local/bin/fake-nsqd
```

---

## 2. 写一个 systemd unit

文件 `/etc/systemd/system/fake-nsqd.service`：

```ini
[Unit]
Description=Fake NSQ Daemon (demo with panic)
After=network.target

[Service]
ExecStart=/usr/local/bin/fake-nsqd
Restart=always
RestartSec=3

StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

---

## 3. 启动并观察

```bash
sudo systemctl daemon-reload
sudo systemctl enable fake-nsqd
sudo systemctl start fake-nsqd
```

查看日志：

```bash
journalctl -u fake-nsqd -f
```

你会看到类似输出：

```
Sep 25 20:12:01 myhost fake-nsqd[12345]: fake-nsqd started...
Sep 25 20:12:03 myhost fake-nsqd[12345]: panic: fake-nsqd: something went wrong!!!
Sep 25 20:12:03 myhost fake-nsqd[12345]: goroutine 1 [running]:
Sep 25 20:12:03 myhost fake-nsqd[12345]: main.main()
Sep 25 20:12:03 myhost fake-nsqd[12345]:     /usr/local/bin/fake-nsqd.go:12 +0x...
Sep 25 20:12:03 myhost systemd[1]: fake-nsqd.service: Main process exited, code=exited, status=2/INVALIDARGUMENT
Sep 25 20:12:03 myhost systemd[1]: fake-nsqd.service: Failed with result 'exit-code'.
Sep 25 20:12:06 myhost systemd[1]: fake-nsqd.service: Scheduled restart job, restart counter is at 1.
Sep 25 20:12:06 myhost systemd[1]: Started Fake NSQ Daemon (demo with panic).
Sep 25 20:12:06 myhost fake-nsqd[12367]: fake-nsqd started...
```

---

## 4. 效果

* 程序 2 秒后 panic → 崩溃。
* systemd 捕获退出码，记录日志。
* 3 秒后自动重启，程序又跑起来。
* 这个过程可以无限循环，保证服务不会“死掉”。

---

✅ 这样你就能完整体验到 **panic → 崩溃日志 → 自动重启** 的流程。
真正的 NSQ 在生产环境里也是依赖这种守护方式。

---

要不要我再帮你扩展一下：在 panic 时把 **完整 stack trace 单独写进文件**，方便后续调试？

