/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 16:34:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 20:39:27
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_models/gorm_user_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice_models

import (
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_config"
	"MLC_GO/pkg/hglog"
	"time"

	"gorm.io/gorm/clause"
)

type GormUser struct {
	Id int64 `json:"id" gorm:"primary_key"`
	Name string `json:"name"`
	Age int32 `json:"age"`
	Sex int8 `json:"sex"`
	Phone string `json:"phone`
	Birthday time.Time `gorm:"column:day_of_the_beast"` // 将列名设为 `day_of_the_beast`
	CreatedAt time.Time // 在创建时，如果该字段值为零值，则使用当前时间填充
	UpdatedAt int       // 在创建时该字段值为零值或者在更新时，使用当前时间戳秒数填充
}

// 插入一条用户数据
func InsertOneUser(user GormUser) (id int64, err error) {
	err = gorm_practice_config.GormDB.Create(&user).Error
	if err != nil {
		hglog.ErrInfo("gorm 插入用户数据失败!!")
		return 0, err
	}
	return user.Id, err
}

// 批量插入用户数据, 参数是引用类型
func BatchInsertUsers(users []GormUser) (ids []int64, err error){
	tx := gorm_practice_config.GormDB.CreateInBatches(users, len(users))
	if tx.Error != nil {
		hglog.ErrInfo("gorm 批量插入用户数据失败!!")
		return []int64{}, tx.Error
	}
	
	ids = []int64{}
	for idx, user := range users {
		ids[idx] = user.Id
	}
	return ids, nil
}

// 插入主键冲突的时候操作(插入处理-安全操作)
func UpsertOp(user GormUser) {
	// 第一种处理情况: 在冲突时，什么都不做(如果冲突，则忽略)
	// 效果: 如果 id 已存在，则不插入数据，不报错。
	gorm_practice_config.GormDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&user)
	
	// 在`id`冲突时，将列更新为默认值(如果冲突，则更新指定字段)
	// 效果:	如果 id 不存在，则正常插入。
	// 		   如果 id 已存在，则 更新 name, age, sex, phone 的值。
	gorm_practice_config.GormDB.Clauses(clause.OnConflict{ 
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string] interface{}{"name": "", "age":0, "sex": 1}),
	}).Create(&user)

	// 在`id`冲突时，将列更新为新值(如果冲突，则更新所有非主键字段
	// 效果：
	// 	如果 id 不存在，则正常插入。
	// 	如果 id 已存在，则 更新除 id 以外的所有字段。
	gorm_practice_config.GormDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "age", "sex", "phone"}),
	}).Create(&user)

	// 在冲突时，更新除主键以外的所有列到新值
	gorm_practice_config.GormDB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&user)	
}

// 根据ID删除
func DeleteUserByUid(id int64) (err error) {
	user := GormUser{
		Id: id,
	}
	err = gorm_practice_config.GormDB.Delete(&user).Error
	// 根据条件删除: constants.GVA_DB.Where("sex = ?", 0).Delete(GormUser{})
	if err != nil {
		hglog.ErrInfo("gorm 删除用户数据失败!!")
		return err
	}
	return nil
}

//根据 id 批量删除数据
func BatchDeleteUserByIds(ids []int64) (err error) {
	if ids == nil || len(ids) == 0 {
		return
	}
	//删除方式1
	err = gorm_practice_config.GormDB.Where("id in ?", ids).Delete(GormUser{}).Error
	if err != nil {
		hglog.ErrInfo("gorm 批量删除数据 DeleteUserById err: ", err)
		return err
	}
	//删除方式 2
	//constants.GVA_DB.Delete(GormUser{}, "id in ?", ids)

	return nil
}

//根据id更新数据，全量字段更新，即使字段是0值
func UpdateUserById(user GormUser) (err error) {
	err = gorm_practice_config.GormDB.Save(&user).Error
	if err != nil {
		hglog.ErrInfo("gorm 更新数据 UpdateUserById err: ", err)
		return err
	}
	return nil
}

//更新指定列
//update user set `columnName` = v where id = id;
func UpdateSpecialColumn(id int64, columnName string, v interface{}) (err error) {
	err = gorm_practice_config.GormDB.Model(&GormUser{Id: id}).Update(columnName, v).Error
	if err != nil {
		hglog.ErrInfo("gorm 更新指定列UpdateSpecialColumn err: ", err)
		return err
	}
	return nil
}


//更新- 根据 `struct` 更新属性，只会更新非零值的字段
//update user set `columnName` = v where id = id;
//当通过 struct 更新时，GORM 只会更新非零字段。 如果您想确保指定字段被更新，你应该使用 Select 更新选定字段，或使用 map 来完成更新操作
func UpdateSelective(user GormUser) (effected int64, err error) {
	
	// 更新非0值的字段：
	tx := gorm_practice_config.GormDB.Model(&user).Updates(&GormUser{
		Id:    user.Id,
		Name:  user.Name,
		Age:   user.Age,
		Sex:   user.Sex,
		Phone: user.Phone,
	})

	/*
	// 如果你想更新0值的字段，那么可以使用 Select 函数先选择指定的列名，或者使用 map 来完成：
	//map 方式会更新0值字段
	tx1 = gorm_practice_config.GormDB.Model(&user).Updates(map[string]interface{}{
		"Id":    user.Id,
		"Name":  user.Name,
		"Age":   user.Age,
		"Sex":   user.Sex,
		"Phone": user.Phone,
  	})

	// Select 方式指定列名：
	//Select 方式指定列名
	tx2 = gorm_practice_config.GormDB.Model(&user).Select("Name", "Age", "Phone").Updates(&GormUser{
		Id:    user.Id,
		Name:  user.Name,
		Age:   user.Age,
		Sex:   user.Sex,
		Phone: user.Phone,
	})

	// Select 选定所有列名：
	// Select 所有字段（查询包括零值字段的所有字段）
	tx3 = gorm_practice_config.GormDB.Model(&user).Select("*").Updates(&GormUser{
		Id:    user.Id,
		Name:  user.Name,
		Age:   user.Age,
		Sex:   user.Sex,
		Phone: user.Phone,
	})

	// Select 排除指定列名：
	// Select 除 Phone 外的所有字段（包括零值字段的所有字段）
	tx4 = gorm_practice_config.GormDB.Model(&user).Select("*").Omit("Phone").Updates(&GormUser{
		Id:    user.Id,
		Name:  user.Name,
		Age:   user.Age,
		Sex:   user.Sex,
		Phone: user.Phone,
	})
	*/
	
	if tx.Error != nil {
		return 0, tx.Error
	}
	return tx.RowsAffected, nil
}

//根据 条件 批量更新
func BatchUpdateByIds(ids []int64, user GormUser) (effected int64, err error) {
	if ids == nil || len(ids) == 0 {
	  return
	}
	tx := gorm_practice_config.GormDB.Model(GormUser{}).Where("id in ?", ids).Updates(&user)
	if tx.Error != nil {
	  return 0, tx.Error
	}
	return tx.RowsAffected, nil
  }


//查询用户信息根据uid
func QueryUserByUid(uid int64) (u GormUser) {
	var user GormUser
	gorm_practice_config.GormDB.Where("id = ?", uid).First(&user)
	return user
}

//查询操作
func queryOp(user GormUser) {

	// 获取第一条记录（主键升序）
	// SELECT * FROM user ORDER BY id LIMIT 1;
	gorm_practice_config.GormDB.First(&user)

	// 获取一条记录，没有指定排序字段
	// SELECT * FROM user LIMIT 1;
	gorm_practice_config.GormDB.Take(&user)

	// 获取最后一条记录（主键降序）
	// SELECT * FROM user ORDER BY id DESC LIMIT 1;
	gorm_practice_config.GormDB.Last(&user)

	// SELECT * FROM user WHERE id = 10;
	gorm_practice_config.GormDB.First(&user, 10)

	// SELECT * FROM user WHERE id = 10;
	gorm_practice_config.GormDB.First(&user, "10")

	// SELECT * FROM user WHERE id IN (1,2,3);
	gorm_practice_config.GormDB.Find(&user, []int{1, 2, 3})

	// 获取全部记录
	// SELECT * FROM user;
	result := gorm_practice_config.GormDB.Find(&user)
	result.Rows()

	// 获取第一条匹配的记录
	// SELECT * FROM user WHERE name = 'xiaoming' ORDER BY id LIMIT 1;
	gorm_practice_config.GormDB.Where("name = ?", "xiaoming").First(&user)

	// 获取全部匹配的记录
	// SELECT * FROM user WHERE name <> 'xiaoming';
	gorm_practice_config.GormDB.Where("name <> ?", "xiaoming").Find(&user)

	// IN
	// SELECT * FROM user WHERE name IN ('xiaoming','xiaohong');
	gorm_practice_config.GormDB.Where("name IN ?", []string{"xiaoming", "xiaohong"}).Find(&user)

	// LIKE
	// SELECT * FROM user WHERE name LIKE '%ming%';
	gorm_practice_config.GormDB.Where("name LIKE ?", "%ming%").Find(&user)

	// AND
	// SELECT * FROM user WHERE name = 'xiaoming' AND age >= 33;
	gorm_practice_config.GormDB.Where("name = ? AND age >= ?", "xiaoming", 33).Find(&user)

	// Time
	// SELECT * FROM user WHERE updated_at > '2021-03-10 15:44:23';
	gorm_practice_config.GormDB.Where("updated_at > ?", "2021-03-10 15:44:23").Find(&user)

	// BETWEEN
	// SELECT * FROM user WHERE created_at BETWEEN ''2021-03-07 15:44:23' AND '2021-03-10 15:44:23';
	gorm_practice_config.GormDB.Where("created_at BETWEEN ? AND ?", "2021-03-07 15:44:23", "2021-03-10 15:44:23").Find(&user)

	// SELECT * FROM user WHERE NOT name = "xiaoming" ORDER BY id LIMIT 1;
	gorm_practice_config.GormDB.Not("name = ?", "xiaoming").First(&user)

	// Not In
	// SELECT * FROM user WHERE name NOT IN ("xiaoming", "xiaohong");
	gorm_practice_config.GormDB.Not(map[string]interface{}{"name": []string{"xiaoming", "xiaohong"}}).Find(&user)

	// Struct
	// SELECT * FROM user WHERE name <> "xiaoming" AND age <> 20 ORDER BY id LIMIT 1;
	gorm_practice_config.GormDB.Not(GormUser{Name: "xiaoming", Age: 20}).First(&user)

	// 不在主键切片中的记录
	// SELECT * FROM user WHERE id NOT IN (1,2,3) ORDER BY id LIMIT 1;
	gorm_practice_config.GormDB.Not([]int64{1, 2, 3}).First(&user)

	// SELECT * FROM user WHERE name = 'xiaoming' OR name = 'xiaohong';
	gorm_practice_config.GormDB.Where("name = ?", "xiaoming").Or("name = ?", "xiaohong").Find(&user)

	// Struct
	// SELECT * FROM user WHERE name = 'xiaoming' OR (name = 'xiaohong' AND age = 20);
	gorm_practice_config.GormDB.Where("name = 'xiaoming'").Or(GormUser{Name: "xiaohong", Age: 20}).Find(&user)

	// Map
	// SELECT * FROM user WHERE name = 'xiaoming' OR (name = 'xiaohong' AND age = 20);
	gorm_practice_config.GormDB.Where("name = 'xiaoming'").Or(map[string]interface{}{"name": "xiaohong", "age": 20}).Find(&user)

	// SELECT name, age FROM user;
	gorm_practice_config.GormDB.Select("name", "age").Find(&user)

	// SELECT name, age FROM user;
	gorm_practice_config.GormDB.Select([]string{"name", "age"}).Find(&user)

	// SELECT COALESCE(age,'20') FROM user;
	gorm_practice_config.GormDB.Table("user").Select("COALESCE(age,?)", 20).Rows()

	// SELECT * FROM user ORDER BY age desc, name;
	gorm_practice_config.GormDB.Order("age desc, name").Find(&user)

	// 多个 order
	// SELECT * FROM user ORDER BY age desc, name;
	gorm_practice_config.GormDB.Order("age desc").Order("name").Find(&user)

	// SELECT * FROM user ORDER BY FIELD(id,1,2,3)
	gorm_practice_config.GormDB.Clauses(clause.OrderBy{
		Expression: clause.Expr{SQL: "FIELD(id,?)", Vars: []interface{}{[]int{1, 2, 3}}, WithoutParentheses: true},
	}).Find(&GormUser{})

	// SELECT * FROM user LIMIT 10;
	gorm_practice_config.GormDB.Limit(10).Find(&user)

	// SELECT * FROM user OFFSET 10;
	gorm_practice_config.GormDB.Offset(10).Find(&user)

	// SELECT * FROM user OFFSET 0 LIMIT 10;
	gorm_practice_config.GormDB.Limit(10).Offset(0).Find(&user)

	// SELECT name, sum(age) as total FROM `users` WHERE name LIKE "ming%" GROUP BY `name`
	gorm_practice_config.GormDB.Model(&GormUser{}).Select("name, sum(age) as total").Where("name LIKE ?", "group%").Group("name").First(&result)

	// SELECT name, sum(age) as total FROM `users` GROUP BY `name` HAVING name = "group"
	gorm_practice_config.GormDB.Model(&GormUser{}).Select("name, sum(age) as total").Group("name").Having("name = ?", "group").Find(&result)

	//SELECT distinct(name, age) from user order by name, age desc
	gorm_practice_config.GormDB.Distinct("name", "age").Order("name, age desc").Find(&user)

}

//事务测试
func TestGormTx(user GormUser) (err error) {
	tx := gorm_practice_config.GormDB.Begin()
	// 注意，一旦你在一个事务中，使用tx作为数据库句柄
	if err := tx.Create(&GormUser{
		Name:  "liliya",
		Age:   13,
		Sex:   0,
		Phone: "15543212346",
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Updates(&GormUser{
		Id:    user.Id,
		Name:  user.Name,
		Age:   user.Age,
		Sex:   user.Sex,
		Phone: user.Phone,
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}
