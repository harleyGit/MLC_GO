package HGRouterPackage

import (
	"fmt"
	"io"
	"strconv"
)

// HGWritePrometheusMetrics 输出固定模块标签的网关计数与单实例并发水位，不访问 Redis 或上游服务。
func (g *HGAPIGateway) HGWritePrometheusMetrics(w io.Writer) {
	if g == nil || !g.enabled {
		return
	}
	_, _ = fmt.Fprint(w, "# HELP mlc_api_gateway_requests_total Requests accepted by the API gateway.\n# TYPE mlc_api_gateway_requests_total counter\n")
	for _, module := range g.modules {
		label := strconv.Quote(module.name)
		_, _ = fmt.Fprintf(w, "mlc_api_gateway_requests_total{module=%s} %d\n", label, module.metrics.requests.Load())
	}
	_, _ = fmt.Fprint(w, "# HELP mlc_api_gateway_rejections_total Requests rejected by gateway reason.\n# TYPE mlc_api_gateway_rejections_total counter\n")
	for _, module := range g.modules {
		label := strconv.Quote(module.name)
		_, _ = fmt.Fprintf(w, "mlc_api_gateway_rejections_total{module=%s,reason=\"rate_limit\"} %d\n", label, module.metrics.rateLimited.Load())
		_, _ = fmt.Fprintf(w, "mlc_api_gateway_rejections_total{module=%s,reason=\"overload\"} %d\n", label, module.metrics.overloaded.Load())
		_, _ = fmt.Fprintf(w, "mlc_api_gateway_rejections_total{module=%s,reason=\"redis_failure\"} %d\n", label, module.metrics.redisFailures.Load())
	}
	_, _ = fmt.Fprint(w, "# HELP mlc_api_gateway_in_flight Current requests inside each module gateway bulkhead.\n# TYPE mlc_api_gateway_in_flight gauge\n")
	for _, module := range g.modules {
		_, _ = fmt.Fprintf(w, "mlc_api_gateway_in_flight{module=%s} %d\n", strconv.Quote(module.name), len(module.inFlight))
	}
}
