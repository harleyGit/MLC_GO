package OpsHandlerPackage

import (
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
	"net/http"
)

// HGCrawlerSpiderStatus 读取数据库权威期望状态；实际运行状态仍从独立 crawler 的 spiders 接口获取。
func (h *Handler) HGCrawlerSpiderStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePackage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if !h.hgAuthorizeCrawler(w, r, userID, "crawler.task.read") {
		return
	}
	if h.crawlerSpiderSwitch == nil {
		hgWriteCrawlerUnavailable(w, r)
		return
	}
	desired, err := h.crawlerSpiderSwitch.HGDesiredRunning(r.Context())
	if err != nil {
		hgWriteSpiderSwitchFailure(w, r)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"id": "bilibili", "desiredRunning": desired})
}

// HGCrawlerSpiderStart 通过已鉴权的主服务提交运行意图；数据库成功不意味着异步 worker 已完成启动。
func (h *Handler) HGCrawlerSpiderStart(w http.ResponseWriter, r *http.Request) {
	h.hgSetCrawlerSpider(w, r, true)
}

// HGCrawlerSpiderStop 通过已鉴权的主服务提交停止意图；worker 在下一轮轮询时取消当前采集。
func (h *Handler) HGCrawlerSpiderStop(w http.ResponseWriter, r *http.Request) {
	h.hgSetCrawlerSpider(w, r, false)
}

func (h *Handler) hgSetCrawlerSpider(w http.ResponseWriter, r *http.Request, desired bool) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePackage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if !h.hgAuthorizeCrawler(w, r, userID, "crawler.task.run") {
		return
	}
	if h.crawlerSpiderSwitch == nil {
		hgWriteCrawlerUnavailable(w, r)
		return
	}
	changed, err := h.crawlerSpiderSwitch.HGSetDesiredRunning(r.Context(), desired, userID)
	if err != nil {
		hgWriteSpiderSwitchFailure(w, r)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"id": "bilibili", "desiredRunning": desired, "changed": changed})
}

func hgWriteSpiderSwitchFailure(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	HGResponsePackage.FailResult[string](w, r, HGResponsePackage.HGErrorResult{Code: HGResponsePackage.InternalError.Code, Message: "爬虫开关状态不可用，请检查迁移和数据库"})
}
