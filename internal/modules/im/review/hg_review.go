// Package review 承载独立 IM 私有附件审核服务；任何探测、扫描或封存失败均不能返回 ready。
package review

import (
	"MLC_GO/internal/modules/im/transport"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const hgMaxReviewBytes = 128 << 20

var hgReviewID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var hgReviewHash = regexp.MustCompile(`^[a-f0-9]{64}$`)
var hgRejected = errors.New("审核拒绝")

// HGConfig 只保留审核专属身份；上传服务的 S3 身份不得拥有 sealed 写权限。
type HGConfig struct {
	Token      string // Go 到审核器的独立 Bearer 密钥，至少 32 字节。
	Bucket     string // 绑定的私有桶名，拒绝请求体中的其他桶。
	ClamdAddr  string // 仅内部网络可达的 ClamAV TCP 地址。
	FFprobe    string // 本机 ffprobe 可执行文件的绝对路径。
	FFmpeg     string // 本机 ffmpeg 绝对路径，视频审核必须实际解码取帧。
	Python     string // 装有固定版本 NudeNet/ONNXRuntime 的隔离 Python 绝对路径。
	Moderator  string // 本仓库提供的本地内容模型脚本绝对路径。
	TempDir    string // 受限临时目录；空值使用系统临时目录，不得对其他用户开放。
	S3Endpoint string // SeaweedFS S3 固定端点。
	S3Region   string // S3 签名 region。
	S3Key      string // 审核专属访问密钥，隔离区只读、封存区条件写入和只读。
	S3Secret   string // 审核专属签名密钥；只在内存保存。
}

// HGService 同时限制并发和文件大小，审查结果只供已认证的 Go 后台任务使用。
type HGService struct {
	cfg     HGConfig               // 启动时校验的配置，不在请求中覆盖。
	storage *transport.HGSeaweedFS // 专用身份读隔离区、条件封存并复核字节。
	slots   chan struct{}          // 单进程至多两个在途审核，满载立即拒绝。
}

// HGNewService 缺少真实扫描器或私有存储配置时拒绝启动，不能退化为只检查扩展名。
func HGNewService(cfg HGConfig) (*HGService, error) {
	if len(cfg.Token) < 32 || cfg.Bucket == "" || cfg.ClamdAddr == "" || !strings.HasPrefix(cfg.FFprobe, "/") || !strings.HasPrefix(cfg.FFmpeg, "/") || !strings.HasPrefix(cfg.Python, "/") || !strings.HasPrefix(cfg.Moderator, "/") {
		return nil, fmt.Errorf("IM 审核服务配置不完整")
	}
	if err := hgValidateClamd(cfg.ClamdAddr); err != nil {
		return nil, err
	}
	for _, executable := range []string{cfg.FFprobe, cfg.FFmpeg, cfg.Python} {
		if info, err := os.Stat(executable); err != nil || info.IsDir() || info.Mode()&0111 == 0 {
			return nil, fmt.Errorf("媒体探测或模型依赖不可执行")
		}
	}
	if info, err := os.Stat(cfg.Moderator); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("内容检测脚本不存在")
	}
	checkCtx, stop := context.WithTimeout(context.Background(), 8*time.Second)
	defer stop()
	check := exec.CommandContext(checkCtx, cfg.Python, cfg.Moderator, "--check")
	check.Env = []string{"PATH=/usr/bin:/bin"}
	if check.Run() != nil {
		return nil, fmt.Errorf("本地内容检测模型未就绪")
	}
	storage, err := transport.HGNewSeaweedFS(cfg.S3Endpoint, cfg.S3Region, cfg.Bucket, cfg.S3Key, cfg.S3Secret)
	if err != nil {
		return nil, err
	}
	if err := hgPingClamd(context.Background(), cfg.ClamdAddr); err != nil {
		storage.HGClose()
		return nil, fmt.Errorf("ClamAV 未就绪: %w", err)
	}
	return &HGService{cfg: cfg, storage: storage, slots: make(chan struct{}, 2)}, nil
}

func (s *HGService) HGClose() { s.storage.HGClose() }

// HGRequest 是 Go 媒体任务到审核进程的协议；键由服务端生成，不能作为任意 S3 对象读写接口。
type HGRequest struct {
	MediaID   string `json:"mediaId"`   // 32 位十六进制随机标识。
	Bucket    string `json:"bucket"`    // 必须与启动期绑定的私有桶一致。
	ObjectKey string `json:"objectKey"` // 只允许 quarantine/<mediaId>。
	SealedKey string `json:"sealedKey"` // 只允许 sealed/<mediaId>。
	SHA256    string `json:"sha256"`    // Go 所记录的原文件摘要，64 位十六进制。
	MIME      string `json:"mime"`      // 原申请 MIME；必须与真实探测类型吻合。
	Size      int64  `json:"size"`      // 原申请字节数，最多 128MiB。
}

// HGHandler 校验固定凭据和请求边界后完成有界审核，业务拒绝与依赖故障严格区分。
func (s *HGService) HGHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost || r.URL.Path != "/review" {
		http.NotFound(w, r)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.cfg.Token)) != 1 {
		http.Error(w, "未授权", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "请求格式错误", http.StatusUnsupportedMediaType)
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		http.Error(w, "审核繁忙", http.StatusServiceUnavailable)
		return
	}
	var cmd HGRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cmd) != nil || decoder.Decode(new(any)) != io.EOF || !s.hgValid(cmd) {
		http.Error(w, "审核请求不合法", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 19*time.Second)
	defer cancel()
	result, err := s.hgReview(ctx, cmd)
	if err != nil && !errors.Is(err, hgRejected) {
		http.Error(w, "审核依赖不可用", http.StatusServiceUnavailable)
		return
	}
	if errors.Is(err, hgRejected) {
		result = transport.HGReviewResult{Status: "rejected"}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// HGReady 仅表示本进程与病毒扫描器可用，不证明 S3 条件封存或内容合规审核通过。
func (s *HGService) HGReady(w http.ResponseWriter, r *http.Request) {
	if err := hgPingClamd(r.Context(), s.cfg.ClamdAddr); err != nil {
		http.Error(w, "扫描依赖不可用", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *HGService) hgValid(cmd HGRequest) bool {
	return hgReviewID.MatchString(cmd.MediaID) && hgReviewHash.MatchString(cmd.SHA256) && cmd.Bucket == s.cfg.Bucket &&
		cmd.ObjectKey == "quarantine/"+cmd.MediaID && cmd.SealedKey == "sealed/"+cmd.MediaID &&
		cmd.Size > 0 && cmd.Size <= hgMaxReviewBytes && hgAllowedMIME(cmd.MIME, cmd.Size)
}

func hgAllowedMIME(mime string, size int64) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return size <= 10<<20
	case "video/mp4", "video/quicktime", "audio/mpeg", "audio/ogg", "audio/mp4", "audio/aac", "application/pdf", "application/zip", "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return true
	default:
		return false
	}
}

// hgReview 仅封存已经读入、哈希匹配、真实探测且由 ClamAV 扫描的字节；封存后重新读取核验。
func (s *HGService) hgReview(ctx context.Context, cmd HGRequest) (transport.HGReviewResult, error) {
	var result transport.HGReviewResult
	reader, err := s.storage.HGReviewOpenQuarantine(ctx, cmd.ObjectKey, cmd.Size)
	if err != nil {
		return result, err
	}
	defer reader.Close()
	file, err := os.CreateTemp(s.cfg.TempDir, "hg-im-review-*")
	if err != nil {
		return result, err
	}
	defer func() { file.Close(); os.Remove(file.Name()) }()
	hash := sha256.New()
	n, err := io.CopyN(io.MultiWriter(file, hash), reader, cmd.Size)
	if err != nil || n != cmd.Size {
		return result, fmt.Errorf("隔离区对象读取不完整")
	}
	var extra [1]byte
	extraCount, extraErr := reader.Read(extra[:])
	if extraCount != 0 || extraErr != io.EOF {
		return result, fmt.Errorf("隔离区对象字节数不匹配")
	}
	if hex.EncodeToString(hash.Sum(nil)) != cmd.SHA256 {
		return result, hgRejected
	}
	if err = file.Sync(); err != nil {
		return result, err
	}
	width, height, duration, err := hgInspect(ctx, s.cfg.FFprobe, file.Name(), cmd.MIME, cmd.Size)
	if err != nil {
		return result, hgRejected
	}
	clean, err := hgScan(ctx, s.cfg.ClamdAddr, file)
	if err != nil {
		return result, err
	}
	if !clean {
		return result, hgRejected
	}
	if strings.HasPrefix(cmd.MIME, "image/") || strings.HasPrefix(cmd.MIME, "video/") {
		if err = s.hgModerate(ctx, file.Name(), cmd.MIME, duration); err != nil {
			return result, err
		}
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	if _, err = s.storage.HGReviewSeal(ctx, cmd.SealedKey, cmd.MIME, cmd.Size, file); err != nil {
		return result, err
	}
	sealed, err := s.storage.HGOpen(ctx, cmd.SealedKey, "")
	if err != nil {
		return result, err
	}
	defer sealed.Body.Close()
	if sealed.ContentLength != cmd.Size {
		return result, fmt.Errorf("封存对象长度不一致")
	}
	check := sha256.New()
	n, err = io.CopyN(check, sealed.Body, cmd.Size)
	if err != nil || n != cmd.Size || hex.EncodeToString(check.Sum(nil)) != cmd.SHA256 {
		return result, fmt.Errorf("封存对象内容不一致")
	}
	extraCount, extraErr = sealed.Body.Read(extra[:])
	if extraCount != 0 || extraErr != io.EOF {
		return result, fmt.Errorf("封存对象有额外字节")
	}
	return transport.HGReviewResult{Status: "ready", SHA256: cmd.SHA256, Size: cmd.Size, MIME: cmd.MIME, SealedKey: cmd.SealedKey, Width: width, Height: height, DurationMS: duration}, nil
}
