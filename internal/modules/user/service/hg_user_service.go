/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 10:54:52
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-15 10:38:02
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_user_service.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 * 功能：业务逻辑层
 */
package UserServicePackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	UserModelsPackage "MLC_GO/internal/models/user_models"
	utilsPackage "MLC_GO/internal/pkg/utils"
	"strings"
)

func RegisterService(account, code, password string) error{
	key := "verify:" + account
	v, err := PersistenceRedisPackage.GetFromRedis(key)
	if err != nil || v != code {
		return err
	}

	salt := utilsPackage.GenerateRandomNum(8)
	u := &UserModelsPackage.HGUserModel{
		PasswordHash: utilsPackage.HashPassword(password, salt),
		Salt:         salt,
	}

	if strings.Contains(account, "@") {
		u.Email = account
		// u.Email.Valid = true
	} else {
		u.Phone = account
		// u.Phone.Valid = true
	}

	err = PersistenceSQLPackage.CreateUser(u)
	if err == nil {
		PersistenceRedisPackage.DeleteFromRedis(key)
	}
	return err
}

func LoginService(account, password string) (string, error) {
	u, err := PersistenceSQLPackage.GetUserByEmail(account)
	if err != nil {
		return "", err
	}

	hashedPassword := utilsPackage.HashPassword(password, u.Salt)
	if hashedPassword != u.PasswordHash {
		return "", err
	}
	token := utilsPackage.GenerateRandomNum(16)
	err = PersistenceRedisPackage.SetToRedis("token:"+token, u.UserID, 24*3600)
	
	return token, nil
}


