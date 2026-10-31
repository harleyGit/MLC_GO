package review

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

func hgValidateClamd(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" || (host != "127.0.0.1" && host != "::1") {
		return fmt.Errorf("ClamAV 仅允许本机 loopback TCP 端点")
	}
	return nil
}

func hgPingClamd(ctx context.Context, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(conn, "zPING\x00"); err != nil {
		return err
	}
	reply, err := bufio.NewReader(io.LimitReader(conn, 16)).ReadString('\x00')
	if err != nil || reply != "PONG\x00" {
		return fmt.Errorf("ClamAV 未响应 PING")
	}
	return nil
}

// hgScan 使用 ClamAV INSTREAM 扫描原始字节；FOUND 为拒绝，其余未知响应都视作依赖故障。
func hgScan(ctx context.Context, addr string, file *os.File) (bool, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, fmt.Errorf("连接 ClamAV 失败")
	}
	defer conn.Close()
	deadline := time.Now().Add(110 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	if _, err := io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return false, fmt.Errorf("启动 ClamAV 扫描失败")
	}
	var chunk [64 << 10]byte
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		n, readErr := file.Read(chunk[:])
		if n > 0 {
			var header [4]byte
			binary.BigEndian.PutUint32(header[:], uint32(n))
			if _, err := conn.Write(header[:]); err != nil {
				return false, fmt.Errorf("ClamAV 数据流失败")
			}
			if _, err := conn.Write(chunk[:n]); err != nil {
				return false, fmt.Errorf("ClamAV 数据流失败")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return false, readErr
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return false, fmt.Errorf("ClamAV 结束流失败")
	}
	reply, err := bufio.NewReader(io.LimitReader(conn, 4096)).ReadString('\x00')
	if err != nil {
		return false, fmt.Errorf("ClamAV 响应不完整")
	}
	if reply == "stream: OK\x00" {
		return true, nil
	}
	if strings.HasPrefix(reply, "stream: ") && strings.HasSuffix(reply, " FOUND\x00") {
		return false, nil
	}
	return false, fmt.Errorf("ClamAV 响应不合法")
}
