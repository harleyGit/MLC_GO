package UserDtoPackage

// HGAddressInput 是收货地址创建/全量修改的 JSON 请求；地区代码由客户端选择，服务端只校验格式与省市前缀，不验证行政区划是否真实存在。
type HGAddressInput struct {
	ReceiverName  string `json:"receiverName"`  // 收件人姓名，必填，最多64个字符。
	ReceiverPhone string `json:"receiverPhone"` // 中国大陆收件手机号，必填，11位。
	Region        string `json:"region"`        // 省市地区展示名称，必填，最多128个字符。
	ProvinceCode  string `json:"provinceCode"`  // 省级行政区划代码，必填，6位数字。
	CityCode      string `json:"cityCode"`      // 市级行政区划代码，必填，6位数字，前两位与省级一致。
	DetailAddress string `json:"detailAddress"` // 街道、门牌等详细地址，必填，最多512个字符。
}

// HGAddress 是用户收货地址响应；仅在本人鉴权通过后返回，不用于订单快照。
type HGAddress struct {
	ID        int64  `json:"id"`        // 地址内部主键，正整数；保留现有接口兼容性。
	AddressID string `json:"addressId"` // 服务端生成的地址业务标识，格式 ADDR_ 加26位 ULID，客户端不可提交。
	HGAddressInput
	IsDefault bool `json:"isDefault"` // 是否为当前默认地址；每人最多一个。
}
