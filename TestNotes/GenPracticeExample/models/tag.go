/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2025-02-27 13:22:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 18:06:20
* @FilePath: /MLC_GO/TestNotes/PracticeGenExample/models/tag.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 标签列表 models
 */
package models

import "github.com/jinzhu/gorm"

// 创建了一个Tag struct{}，用于Gorm的使用。并给予了附属属性json，这样子在c.JSON的时候就会自动转换格式，非常的便利
type Tag struct {
	Model

	Name string `json:"name"`
	CreatedBy string `json:"created_by"`
	ModifiedBy string `json:"modified_by"`
	State int `json:"state"`
}

func GetTags(pageNum int, pageSize int, maps interface{}) ([]Tag, error){
	var (
		tags []Tag
		err  error
	)

	if pageSize > 0 && pageNum > 0 {
		// db.Where(maps): 根据传入的 maps 参数设定查询条件。
		// 		这里 maps 可以是一个条件的 map，例如 { "state": 1 }，用来过滤符合条件的记录。
		// .Find(&tags)
		// 		执行查询，并将结果存入 tags 这个切片中。此处的查询会根据前面设定的条件进行。
		// .Offset(pageNum)
		// 		设置查询结果的“偏移量”。也就是跳过前面 pageNum 个记录。
		// 		通常在分页时，偏移量的计算方式为 (pageNum-1)*pageSize，不过这里直接使用 pageNum，可能在实际使用中需要注意计算方式是否符合预期。
		// Limit(pageSize)
		// 		限制返回的记录数为 pageSize 条。这样就实现了每页显示固定数量记录的效果。
		err = db.Where(maps).Find(&tags).Offset(pageNum).Limit(pageSize).Error
	} else {
		// db.Where(maps).Find(&tags)
		// 		同样根据 maps 条件查询数据，但这里没有设置分页相关的偏移量和记录限制，因此会查询所有符合条件的记录。
		err = db.Where(maps).Find(&tags).Error
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return tags, nil
}
/*
	可能会有的初学者看到return，而后面没有跟着变量，会不理解；
	其实你可以看到在函数末端，我们已经显示声明了返回值，这个变量在函数体内也可以直接使用，因为他在一开始就被声明了

	有人会疑惑db是哪里来的；因为在同个models包下，因此db *gorm.DB是可以直接使用的
*/
//Deprecated: func GetTags_v1(pageNum int, pageSize int, maps interface{}) (tags []Tag) 废弃了,用 func GetTags(pageNum int, pageSize int, maps interface{}) ([]Tag, error)
func GetTags_v1(pageNum int, pageSize int, maps interface{}) (tags []Tag){
	db.Where(maps).Offset(pageNum).Limit(pageSize).Find(&tags)

	return
}

func ExistTagByID(id int) (bool, error) {
	var tag Tag
	/* 解析每一部分：
		1. db.Select("id").Where("id =? AND deleted_on = ?", id, 0).First(&tag).Error
			db.Select("id")：这是在查询时指定要选择的字段，这里仅选择 id 字段。Select 用于指定查询的字段，避免查询不必要的字段，从而提高性能。
			Where("id =? AND deleted_on = ?", id, 0)：这是查询条件，表示 id 等于指定的值 id，并且 deleted_on 字段值为 0（假设 deleted_on 表示是否删除的标志，0 表示没有删除）。这里使用了 ? 占位符来传递查询参数。
			First(&tag)：这会查询出符合条件的第一条记录，并将结果存入 tag 变量中。First 方法会按照默认的排序（通常是 ID 顺序）返回第一条记录。如果没有找到记录，它会返回一个 gorm.ErrRecordNotFound 错误。
			.Error：返回的错误对象，如果查询成功并且没有遇到其他问题，Error 会是 nil；如果有错误，则包含具体的错误信息。
		
		2. if err != nil && err != gorm.ErrRecordNotFound
			这个条件检查是否发生了查询错误。如果 err 不是 nil 且不是 gorm.ErrRecordNotFound（即没有找到记录），那么说明发生了其他错误，返回 false 和错误信息。
		
		3. if tag.ID > 0
			这个条件检查是否查询到有效的 tag 记录。如果 tag.ID > 0，表示查询到的 tag 存在且有效，返回 true 和 nil（表示没有错误）。如果 ID 小于等于 0，表示没有找到符合条件的记录。 
	*/
	err := db.Select("id").Where("id =? AND deleted_on = ?", id, 0).First(&tag).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return false, err
	}

	if tag.ID > 0 {
		return true, nil
	}

	return false, nil
}

func GetTagTotal(maps interface{}) (count int) {
	/* 
		1. db.Model(&Tag{})
		db: 这是 GORM 的数据库连接对象，提供了数据库操作的功能。
		Model(&Tag{}): 指定了查询的目标表为 Tag 表。这里的 &Tag{} 是一个空的结构体指针，告诉 GORM 操作的是与 Tag 相关的表。
		2. Where(maps)
		Where(maps)：这是查询的条件，maps 是一个 map[string]interface{} 类型的变量，包含了多个字段的条件。例如，你可以使用 map 来动态构建查询条件。
	*/
	db.Model(&Tag{}).Where(maps).Count(&count)

	return
}

func ExistTagByName(name string) bool {
	var tag Tag
	db.Select("id").Where("name = ?", name).First(&tag)
	if tag.ID > 0 {
		return true
	}

	return false
}

func AddTag(name string, state int, createdBy string) bool {
	db.Create(&Tag {
		Name: name,
		State: state,
		CreatedBy: createdBy,
	})

	return true
}

// 硬删除tag代码
func CleanAllTag() bool {
	db.Unscoped().Where("deleted_on != ?", 0).Delete(&Tag{})

	return true
}