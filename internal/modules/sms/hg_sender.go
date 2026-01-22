/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-21 21:21:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-21 21:26:19
* @FilePath: /MLC_GO/internal/modules/sms/hg_sender.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 发送验证码
 */
package HGSMSPackage

import "MLC_GO/internal/pkg/logHG"

type HGSender interface {
	Send(phone, code string) error
}

// TODO: 阿里云 / 腾讯云 替换 
type HGPhoneSMSSender struct {}

func (s *HGPhoneSMSSender) Send(phone, code string) error {
	logHG.DebugFInfo("【手机短信】 phone=%s code=%s \n", phone, code)
	return nil
}