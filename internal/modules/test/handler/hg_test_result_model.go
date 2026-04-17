package HGTestHandlerPackage

import HGResponsePakcage "MLC_GO/internal/response"

type HGTestResultModel struct {
	User string `json:"user"`
	Age  int64  `json:"age"`
}

func (model *HGTestResultModel) ResponseCode() HGResponsePakcage.HGErrorCode {
	return HGResponsePakcage.OKCode
}

func (model *HGTestResultModel) ResponseMessage() string {
	return "success💯"
}
