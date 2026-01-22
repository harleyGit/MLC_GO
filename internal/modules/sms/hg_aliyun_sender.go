/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 21:21:29
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-22 21:27:15
 * @FilePath: /MLC_GO/internal/modules/sms/hg_aliyun_sender.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGSMSPackage

type HGAliyunSender struct {
	AccessKey string
	Secret string
}

type HGMockerSender struct {
	AccessKey string
	Secret string
}

func (s *HGAliyunSender) Send(phone, code string) error {
	// TODO：真实环境
	// 1.构造请求
	// 2. 签名
	// 3.调用阿里云 SMS API
	// 4.判断返回码

	return nil
}