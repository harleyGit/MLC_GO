package platform

import (
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var hgBVIDPattern = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)

// HGValidBVID 限制消息中的视频标识，禁止将任意 URL 带入详情客户端。
func HGValidBVID(value string) bool { return hgBVIDPattern.MatchString(value) }

// hgDetailResponse 解码 view 协议字段，上游 DTO 不直接充当数据库模型。
type hgDetailResponse struct {
	Code int `json:"code"` // 0 为成功。
	Data struct {
		BVID     string `json:"bvid"`     // 视频标识。
		AID      int64  `json:"aid"`      // 稿件编号。
		Title    string `json:"title"`    // 原始标题。
		Desc     string `json:"desc"`     // 原始简介。
		Pic      string `json:"pic"`      // 封面 URL。
		TName    string `json:"tname"`    // 分区名。
		Duration int64  `json:"duration"` // 秒。
		Pubdate  int64  `json:"pubdate"`  // Unix 秒。
		Owner    struct {
			MID  int64  `json:"mid"`  // 作者编号。
			Name string `json:"name"` // 昵称。
			Face string `json:"face"` // 头像 URL。
		} `json:"owner"` // 作者公开信息。
		Stat struct {
			View     int64 `json:"view"`     // 播放次数。
			Like     int64 `json:"like"`     // 点赞次数。
			Coin     int64 `json:"coin"`     // 投币枚数。
			Favorite int64 `json:"favorite"` // 收藏次数。
			Share    int64 `json:"share"`    // 分享次数。
			Reply    int64 `json:"reply"`    // 评论条数。
			Danmaku  int64 `json:"danmaku"`  // 弹幕条数。
		} `json:"stat"` // 原站累计值。
		Pages []struct {
			CID      int64  `json:"cid"`      // 分 P 编号。
			Page     int    `json:"page"`     // 分 P 序号。
			Part     string `json:"part"`     // 分 P 标题。
			Duration int64  `json:"duration"` // 秒。
		} `json:"pages"` // 原始分 P 列表。
	} `json:"data"` // 视频详情。
}

// HGCompactBilibiliDetail 丢弃 view 接口中流水线不使用的字段，避免原始响应超过 Kafka 单消息预算。
// 输出仍保留 HGCleanBilibiliDetail 所需的协议字段和完整校验入口，不改变后续 raw 阶段语义。
func HGCompactBilibiliDetail(body []byte) ([]byte, error) {
	if len(body) > hgBilibiliMaxResponseBytes || !utf8.Valid(body) {
		return nil, errors.New("详情响应大小或编码无效")
	}
	var hgResponse hgDetailResponse
	if hgErr := json.Unmarshal(body, &hgResponse); hgErr != nil {
		return nil, fmt.Errorf("解析视频详情: %w", hgErr)
	}
	hgCompacted, hgErr := json.Marshal(hgResponse)
	if hgErr != nil {
		return nil, fmt.Errorf("压缩视频详情: %w", hgErr)
	}
	if len(hgCompacted) > 1<<20 {
		return nil, errors.New("详情必要字段超过1MiB消息预算")
	}
	return hgCompacted, nil
}

// HGCleanBilibiliDetail 将 B 站返回的复杂 JSON 转换成自己系统统一的数据结构，同时校验数据的完整性和正确性。
/*整体流程：
B站视频详情 API
       |
       v
原始 JSON 数据
       |
       v
HGCleanBilibiliDetail()
       |
       v
标准化 HGVideoDetail
       |
       v
数据库保存视频详情
       |
       v
Kafka HGStored 事件
       |
       v
hgHistory()
       |
       v
提取播放、点赞、收藏等统计数据
       |
       v
批量写入历史数据库
*/
func HGCleanBilibiliDetail(body []byte, bvid string, collectedAt time.Time) (CrawlerModelPackage.HGVideoDetail, error) {
	var result CrawlerModelPackage.HGVideoDetail
	var response hgDetailResponse
	// 验证JSON 数据是否超过允许大小 ｜｜ 是否为合法 UTF-8 编码
	if len(body) > hgBilibiliMaxResponseBytes || !utf8.Valid(body) {
		return result, errors.New("详情响应大小或编码无效")
	}
	// JSON反序列化，将B站JSON转换为Go结构体
	if err := json.Unmarshal(body, &response); err != nil {
		return result, fmt.Errorf("解析视频详情: %w", err)
	}
	// 验证七个统计字段是否存在【临时定义校验结构，不必单独声明类型】
	var hgRequired struct {
		Code *int `json:"code"` // 区分字段不存在和数值为 0
		Data struct {
			Stat map[string]json.RawMessage `json:"stat"` // 保存 JSON 原始字段值【上游包含evaluation等非计数字段，仅校验约定的七项统计。】
		} `json:"data"` // 校验统计字段存在性。
	}
	if err := json.Unmarshal(body, &hgRequired); err != nil || hgRequired.Code == nil {
		return result, errors.New("详情缺少业务状态")
	}
	for _, hgKey := range []string{"view", "like", "coin", "favorite", "share", "reply", "danmaku"} {
		var hgCount *int64
		// 目的：防止上游接口缺失字段时，错误地将已有播放量、点赞量等覆盖成 0。
		if hgErr := json.Unmarshal(hgRequired.Data.Stat[hgKey], &hgCount); hgErr != nil || hgCount == nil {
			return result, fmt.Errorf("详情缺少统计字段%s", hgKey)
		}
	}
	d := response.Data
	// 校验视频身份和业务数据
	if response.Code != 0 || !HGValidBVID(bvid) || d.BVID != bvid || d.AID <= 0 || d.Owner.MID <= 0 || d.Pubdate <= 0 || collectedAt.IsZero() {
		return result, errors.New("视频详情身份、业务码或时间无效")
	}
	result = CrawlerModelPackage.HGVideoDetail{BVID: bvid, AID: d.AID, Title: strings.TrimSpace(d.Title), Description: strings.TrimSpace(d.Desc),
		CoverURL: hgNormalizeHTTPS(d.Pic), AuthorID: strconv.FormatInt(d.Owner.MID, 10), AuthorName: strings.TrimSpace(d.Owner.Name),
		AuthorAvatar: hgNormalizeHTTPS(d.Owner.Face), Category: strings.TrimSpace(d.TName), Duration: d.Duration,
		PublishedAt: time.Unix(d.Pubdate, 0).UTC(), CollectedAt: collectedAt.UTC(),
		// Stats 字段保存播放量、点赞量、投币量等七项统计数据。
		Stats: CrawlerModelPackage.HGVideoStats{View: d.Stat.View, Like: d.Stat.Like, Coin: d.Stat.Coin, Favorite: d.Stat.Favorite, Share: d.Stat.Share, Reply: d.Stat.Reply, Danmaku: d.Stat.Danmaku}}
	for _, p := range d.Pages { // 处理多 P 视频；B 站一个视频可能包含多个分 P。
		result.Parts = append(result.Parts, CrawlerModelPackage.HGVideoPart{CID: strconv.FormatInt(p.CID, 10), Page: p.Page, Title: strings.TrimSpace(p.Part), Duration: p.Duration})
	}
	return result, HGValidateVideoDetail(result)
}

// HGValidateVideoDetail 在入库消息边界再次校验，防止异常重放绕过清洗。
func HGValidateVideoDetail(d CrawlerModelPackage.HGVideoDetail) error {
	if !HGValidBVID(d.BVID) || d.AID <= 0 || d.Title == "" || d.AuthorName == "" || d.CollectedAt.IsZero() || d.PublishedAt.IsZero() || d.Duration < 0 || d.Duration > int64(^uint32(0)) {
		return errors.New("视频详情必填字段无效")
	}
	if id, err := strconv.ParseInt(d.AuthorID, 10, 64); err != nil || id <= 0 {
		return errors.New("作者编号无效")
	}
	texts := []struct {
		value   string
		maximum int
	}{{d.Title, 255}, {d.Description, 16384}, {d.AuthorName, 255}, {d.Category, 128}}
	for _, field := range texts {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) > field.maximum {
			return errors.New("视频详情文本超过存储限制")
		}
	}
	for _, raw := range []string{d.CoverURL, d.AuthorAvatar} {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(raw) > 1024 || !(u.Hostname() == "hdslb.com" || strings.HasSuffix(u.Hostname(), ".hdslb.com")) {
			return errors.New("视频图片地址无效")
		}
	}
	for _, count := range []int64{d.Stats.View, d.Stats.Like, d.Stats.Coin, d.Stats.Favorite, d.Stats.Share, d.Stats.Reply, d.Stats.Danmaku} {
		if count < 0 {
			return errors.New("原站累计计数为负，拒绝覆盖")
		}
	}
	if len(d.Parts) == 0 || len(d.Parts) > 1000 {
		return errors.New("分P数量必须为1至1000")
	}
	seen := make(map[string]bool, len(d.Parts))
	for i, p := range d.Parts {
		cid, err := strconv.ParseInt(p.CID, 10, 64)
		if err != nil || cid <= 0 || seen[p.CID] || p.Page != i+1 || p.Duration < 0 || p.Duration > int64(^uint32(0)) || !utf8.ValidString(p.Title) || utf8.RuneCountInString(p.Title) > 255 {
			return errors.New("视频分P字段无效或重复")
		}
		seen[p.CID] = true
	}
	return nil
}
