package VideoDanmakuServicePackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	VideoDanmakuRepositoryPackage "MLC_GO/internal/modules/video_danmaku/repository"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	HGDefaultWindowMS uint32 = 60_000
	HGMaxWindowMS     uint32 = 300_000
	HGDefaultPageSize        = 200
	HGMaxPageSize            = 1000
	HGWebSocketPath          = "/api/v1/video_danmaku/ws"
)

var (
	ErrInvalidTarget     = errors.New("弹幕视频不能为空")
	ErrInvalidContent    = errors.New("弹幕内容长度必须为 1 到 100 个字符")
	ErrInvalidRequestID  = errors.New("弹幕请求幂等标识长度必须为 1 到 64 个字符")
	ErrInvalidProgress   = errors.New("弹幕播放位置无效")
	ErrInvalidStyle      = errors.New("弹幕样式无效")
	ErrInvalidWindow     = errors.New("弹幕时间窗必须大于 0 且不超过 5 分钟")
	ErrInvalidPageSize   = errors.New("弹幕分页大小必须为 1 到 1000")
	ErrInvalidCursor     = errors.New("弹幕游标无效")
	ErrTicketUnavailable = errors.New("弹幕连接票据无效或已过期")
)

type hgRepository interface {
	Create(context.Context, VideoDanmakuRepositoryPackage.HGCreateCommand) (VideoDanmakuRepositoryPackage.HGDanmaku, bool, error)
	List(context.Context, string, uint32, uint32, VideoDanmakuRepositoryPackage.HGCursor, int) (VideoDanmakuRepositoryPackage.HGListResult, error)
	ResolveVideo(context.Context, string) (string, error)
}

type hgPublisher interface {
	Publish(context.Context, VideoDanmakuDtoPackage.DanmakuResponse) error
}

// HGTicket 是 Redis 中短期保存的 WebSocket 身份和房间绑定。
type HGTicket struct {
	UserID  string `json:"userId"`
	VideoID string `json:"videoId"`
}

// Service 负责弹幕校验、时间窗游标和一次性 WebSocket 票据。
type Service struct {
	repo      hgRepository
	redis     *PersistenceRedisPackage.RedisService
	ticketTTL time.Duration
	publisher hgPublisher
}

// NewService 创建弹幕服务；ticketTTL 控制浏览器发起 WebSocket 握手的最长期限。
func NewService(repo hgRepository, redis *PersistenceRedisPackage.RedisService, ticketTTL time.Duration) *Service {
	return &Service{repo: repo, redis: redis, ticketTTL: ticketTTL}
}

// SetPublisher 注入提交后实时广播器；持久化成功是主结果，广播失败由历史时间窗恢复。
func (s *Service) SetPublisher(publisher hgPublisher) { s.publisher = publisher }

// Create 校验输入并同步持久化；调用方只能广播成功返回的权威记录。
func (s *Service) Create(ctx context.Context, userID string, req VideoDanmakuDtoPackage.CreateRequest) (VideoDanmakuDtoPackage.DanmakuResponse, error) {
	req.VideoID, req.Content, req.RequestID = strings.TrimSpace(req.VideoID), strings.TrimSpace(req.Content), strings.TrimSpace(req.RequestID)
	if userID == "" || req.VideoID == "" {
		return VideoDanmakuDtoPackage.DanmakuResponse{}, ErrInvalidTarget
	}
	if count := utf8.RuneCountInString(req.Content); count < 1 || count > 100 {
		return VideoDanmakuDtoPackage.DanmakuResponse{}, ErrInvalidContent
	}
	if count := utf8.RuneCountInString(req.RequestID); count < 1 || count > 64 {
		return VideoDanmakuDtoPackage.DanmakuResponse{}, ErrInvalidRequestID
	}
	if req.ProgressMS > 24*60*60*1000 {
		return VideoDanmakuDtoPackage.DanmakuResponse{}, ErrInvalidProgress
	}
	mode, color, fontSize, err := hgStyle(req.Mode, req.Color, req.FontSize)
	if err != nil {
		return VideoDanmakuDtoPackage.DanmakuResponse{}, err
	}
	item, created, err := s.repo.Create(ctx, VideoDanmakuRepositoryPackage.HGCreateCommand{DanmakuID: UtilsPackage.GenerateBusinessID("DMK"), VideoID: req.VideoID, UserID: userID, RequestID: req.RequestID, Content: req.Content, ProgressMS: req.ProgressMS, Mode: mode, Color: color, FontSize: fontSize})
	if err != nil {
		return VideoDanmakuDtoPackage.DanmakuResponse{}, err
	}
	response := hgResponse(item)
	// 幂等重放只返回原权威记录，不再次广播；否则网络超时重试会让其他客户端重复看到同一弹幕。
	if created && s.publisher != nil {
		_ = s.publisher.Publish(ctx, response)
	}
	return response, nil
}

// List 限制时间窗和返回数量，并以 (progress_ms,id) 作为稳定 keyset。
func (s *Service) List(ctx context.Context, req VideoDanmakuDtoPackage.ListRequest) (VideoDanmakuDtoPackage.ListResponse, error) {
	req.VideoID = strings.TrimSpace(req.VideoID)
	if req.VideoID == "" {
		return VideoDanmakuDtoPackage.ListResponse{}, ErrInvalidTarget
	}
	if req.ToMS == 0 {
		req.ToMS = req.FromMS + HGDefaultWindowMS
	}
	if req.ToMS <= req.FromMS || req.ToMS-req.FromMS > HGMaxWindowMS {
		return VideoDanmakuDtoPackage.ListResponse{}, ErrInvalidWindow
	}
	if req.PageSize == 0 {
		req.PageSize = HGDefaultPageSize
	}
	if req.PageSize < 1 || req.PageSize > HGMaxPageSize {
		return VideoDanmakuDtoPackage.ListResponse{}, ErrInvalidPageSize
	}
	cursor, err := hgDecodeCursor(req.Cursor)
	if err != nil {
		return VideoDanmakuDtoPackage.ListResponse{}, err
	}
	page, err := s.repo.List(ctx, req.VideoID, req.FromMS, req.ToMS, cursor, req.PageSize+1)
	if err != nil {
		return VideoDanmakuDtoPackage.ListResponse{}, err
	}
	items := page.Danmaku
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	result := VideoDanmakuDtoPackage.ListResponse{Danmaku: make([]VideoDanmakuDtoPackage.DanmakuResponse, 0, len(items)), HasMore: hasMore, TotalCount: page.TotalCount}
	for _, item := range items {
		result.Danmaku = append(result.Danmaku, hgResponse(item))
	}
	if hasMore {
		result.NextCursor = hgEncodeCursor(items[len(items)-1])
	}
	return result, nil
}

// IssueTicket 校验视频后创建 32 字节随机、短期且单次消费的 WebSocket 票据。
func (s *Service) IssueTicket(ctx context.Context, userID, videoID string) (VideoDanmakuDtoPackage.TicketResponse, error) {
	videoID = strings.TrimSpace(videoID)
	if userID == "" || videoID == "" {
		return VideoDanmakuDtoPackage.TicketResponse{}, ErrInvalidTarget
	}
	if _, err := s.repo.ResolveVideo(ctx, videoID); err != nil {
		return VideoDanmakuDtoPackage.TicketResponse{}, err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return VideoDanmakuDtoPackage.TicketResponse{}, fmt.Errorf("generate danmaku ticket: %w", err)
	}
	ticket := hex.EncodeToString(random)
	if err := s.redis.SetToRedisV2(PersistenceRedisPackage.GetVideoDanmakuTicketKey(ticket), HGTicket{UserID: userID, VideoID: videoID}, s.ticketTTL, ctx); err != nil {
		return VideoDanmakuDtoPackage.TicketResponse{}, fmt.Errorf("store danmaku ticket: %w", err)
	}
	return VideoDanmakuDtoPackage.TicketResponse{Ticket: ticket, WebSocketPath: HGWebSocketPath, ExpiresIn: int64(s.ticketTTL.Seconds())}, nil
}

// ConsumeTicket 通过 Lua 原子获取并删除票据，保证连接握手不能重放。
func (s *Service) ConsumeTicket(ctx context.Context, ticket string) (HGTicket, error) {
	if s.redis == nil || s.redis.Client() == nil {
		return HGTicket{}, ErrTicketUnavailable
	}
	ticket = strings.TrimSpace(ticket)
	if len(ticket) != 64 {
		return HGTicket{}, ErrTicketUnavailable
	}
	if _, err := hex.DecodeString(ticket); err != nil {
		return HGTicket{}, ErrTicketUnavailable
	}
	value, err := s.redis.Client().Eval(ctx, PersistenceRedisPackage.VideoDanmakuConsumeTicketLuaScript, []string{PersistenceRedisPackage.GetVideoDanmakuTicketKey(ticket)}).Text()
	if err != nil || value == "" {
		return HGTicket{}, ErrTicketUnavailable
	}
	var binding HGTicket
	if json.Unmarshal([]byte(value), &binding) != nil || binding.UserID == "" || binding.VideoID == "" {
		return HGTicket{}, ErrTicketUnavailable
	}
	return binding, nil
}

type hgCursorPayload struct {
	ProgressMS uint32 `json:"p"`
	ID         uint64 `json:"i"`
}

func hgEncodeCursor(item VideoDanmakuRepositoryPackage.HGDanmaku) string {
	payload, _ := json.Marshal(hgCursorPayload{ProgressMS: item.ProgressMS, ID: item.ID})
	return base64.RawURLEncoding.EncodeToString(payload)
}
func hgDecodeCursor(value string) (VideoDanmakuRepositoryPackage.HGCursor, error) {
	if strings.TrimSpace(value) == "" {
		return VideoDanmakuRepositoryPackage.HGCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return VideoDanmakuRepositoryPackage.HGCursor{}, ErrInvalidCursor
	}
	var cursor hgCursorPayload
	if json.Unmarshal(payload, &cursor) != nil || cursor.ID == 0 {
		return VideoDanmakuRepositoryPackage.HGCursor{}, ErrInvalidCursor
	}
	return VideoDanmakuRepositoryPackage.HGCursor{ProgressMS: cursor.ProgressMS, ID: cursor.ID}, nil
}
func hgStyle(mode, color string, fontSize uint8) (string, string, uint8, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "scroll"
	}
	if mode != "scroll" && mode != "top" && mode != "bottom" {
		return "", "", 0, ErrInvalidStyle
	}
	color = strings.ToUpper(strings.TrimSpace(color))
	if color == "" {
		color = "#FFFFFF"
	}
	if len(color) != 7 || color[0] != '#' {
		return "", "", 0, ErrInvalidStyle
	}
	if _, err := strconv.ParseUint(color[1:], 16, 24); err != nil {
		return "", "", 0, ErrInvalidStyle
	}
	if fontSize == 0 {
		fontSize = 25
	}
	if fontSize < 12 || fontSize > 36 {
		return "", "", 0, ErrInvalidStyle
	}
	return mode, color, fontSize, nil
}
func hgResponse(item VideoDanmakuRepositoryPackage.HGDanmaku) VideoDanmakuDtoPackage.DanmakuResponse {
	return VideoDanmakuDtoPackage.DanmakuResponse{DanmakuID: item.DanmakuID, SubmissionID: item.SubmissionID, VideoID: item.VideoID, Content: item.Content, ProgressMS: item.ProgressMS, Mode: item.Mode, Color: item.Color, FontSize: item.FontSize, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano)}
}
