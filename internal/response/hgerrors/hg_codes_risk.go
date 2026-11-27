package hgerrors

const (
	HGRiskActionRejected       Code = 2201001
	HGRiskVerificationRequired Code = 2201002
	HGRiskRateLimited          Code = 2201003
)

var hgRisk = []Definition{
	{HGRiskActionRejected, "RISK_ACTION_REJECTED", "操作未通过安全校验", 403, false},
	{HGRiskVerificationRequired, "RISK_VERIFICATION_REQUIRED", "需要额外验证", 403, false},
	{HGRiskRateLimited, "RISK_RATE_LIMITED", "触发风控限流", 429, true},
}
