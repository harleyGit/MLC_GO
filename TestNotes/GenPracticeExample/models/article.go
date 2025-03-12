/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-28 20:11:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-12 19:59:40
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/models/article.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

type Article struct {
	Model
	// gorm:index，用于声明这个字段为索引，如果你使用了自动迁移功能则会有所影响，在不使用则无影响
	TagID int `json:"tag_id" gorm:"index"`
	// Tag字段，实际是一个嵌套的struct，它利用TagID与Tag模型相互关联，在执行查询的时候，能够达到Article、Tag关联查询的功能
	Tag Tag `json:"tag"`

	Title string `json:"title"`
	Desc string `json:"desc"`
	Content string `json:"content"`
	CoverImageUrl string `json:"cover_image_url"`
	CreatedBy string `json:"created_by"`
	ModifiedBy string `json:"modified_by"`
	State int `json:"state"`
}


func ExistArticleByID(id int) (bool, error) {
	var article Article

	err := db.Select("id").Where("id = ? AND deleted_on = ?", id, 0).First(&article).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return false, err
	}

	if article.ID > 0 {
		return true, nil
	}

	return false, nil
}
// 检查数据库中是否存在某个 ID 的文章，如果存在则返回 true，否则返回 false
// Deprecated: 该方法废弃了,不再使用,请用 func ExistArticleByIDV1(id int) (bool, error)
func ExistArticleByIDV1(id int) bool {
	var article Article
	/*
	.Where("id = ?", id)
		使用 WHERE 语句筛选 id 等于传入的参数，? 是占位符，避免 SQL 注入
	
	.First(&article)
		查询 第一条符合条件的记录，并将结果存入 article 结构体
	*/
	db.Select("id").Where("id = ?", id).First(&article)

	if article.ID > 0 {
		return true
	}

	return false
}

// 计算符合条件的文章数量，并返回总数
// maps interface{}：查询条件（可以是 map[string]interface{} 或 struct）
func GetArticleTotal(maps interface {}) (count int) {
	/*
	db.Model(&Article{})：设置查询的表模型 Article。
	.Where(maps)：筛选符合 maps 条件的文章。
	.Count(&count)：统计符合条件的文章数量，并存入 count
	*/
	db.Model(&Article{}).Where(maps).Count(&count)

	return
}

/*
分页获取符合条件的文章列表，并返回文章数组。
参数：
pageNum int：分页起始偏移量（从第几条数据开始）。
pageSize int：每页获取的数据量（一次查询多少条数据）。
maps interface{}：查询条件（筛选文章）。
*/
func GetArticles(pageNum int, pageSize int, maps interface {}) (articles []Article) {
	/*
	db.Preload("Tag")：
		预加载 Tag 关联（预加载文章的分类/标签，避免 N+1 查询）

	.Offset(pageNum)：
		分页偏移量，表示从哪一条记录开始查询。
	
		.Limit(pageSize)：
			查询条数，最多返回 pageSize 条数据。

	Preload是什么东西，为什么查询可以得出每一项的关联Tag？
		Preload就是一个预加载器，它会执行两条 SQL，分别是SELECT * FROM blog_articles;和SELECT * FROM blog_tag WHERE id IN (1,2,3,4);
		那么在查询出结构后，gorm内部处理对应的映射逻辑，将其填充到Article的Tag中，会特别方便，并且避免了循环查询
	*/
	db.Preload("Tag").Where(maps).Offset(pageNum).Limit(pageSize).Find(&articles)


	/*为啥没有返回任何数据？
	
	(count int) 这里的 count 是命名返回值，在函数内部可以直接修改 count 的值。
	db.Model(&Article{}).Where(maps).Count(&count) 直接修改了 count 变量。
	return 语句 省略了变量，默认返回 count。
	*/
	return
}


/*Article是如何关联到Tag？
首先是gorm本身做了大量的约定俗成

Article有一个结构体成员是TagID，就是外键。gorm会通过类名+ID 的方式去找到这两个类之间的关联关系
Article有一个结构体成员是Tag，就是我们嵌套在Article里的Tag结构体，我们可以通过Related进行关联查询
*/
func GetArticle(id int) (article Article) {
	db.Where("id = ?", id).First(&article)
	db.Model(&article).Related(&article.Tag)

	return
}

// data interface{}：包含要更新的字段和值的 map 或结构体。
func EditArticle(id int, data interface {}) error {
	/*
	db.Model(&Article{})：指定要更新的是 Article 表的数据。
	.Where("id = ?", id)：筛选出 id 等于 id 的那一条记录。
	.Updates(data)：将 data 传入的字段和值应用到筛选出的记录中。
	*/
	if err := db.Model(&Article{}).Where("id = ? AND deleted_on = ?", id, 0).Updates(data).Error; err != nil {
		return err
	}

	return nil
}

func AddArticle(data map[string]interface {}) error {
	/*
	db.Create(&Article{})
		db：这是 GORM 的数据库连接对象，允许你执行数据库操作。Create() 方法用于在数据库中插入新纪录。
		&Article{}：这个参数是一个 Article 结构体的指针，它会通过 GORM 插入一条新的数据记录到数据库。这个结构体应该与数据库中的 Article 表结构相对应。	
	
	data["key"].(type)
		data 是一个 map[string]interface{} 类型的变量，它包含了文章的各个字段及其值。
		data["tag_id"].(int)：通过类型断言（.(int)）从 data 中获取 tag_id，并将其转换为 int 类型。
	*/
	article := Article {
		TagID : data["tag_id"].(int),
		Title: data["title"].(string),
		Desc: data["desc"].(string),
		Content: data["content"].(string),
		CreatedBy: data["created_by"].(string),
		State: data["state"].(int),
		CoverImageUrl: data["cover_image_url"].(string),
	}
	if err := db.Create(&article).Error; err != nil {
		return err
	}
	
	return nil
}

func DeleteArticle(id int) bool {
	/*
	Where("id = ?", id): 这个方法用于构建 查询条件，id = ? 表示查询 id 等于传入的 id 值的记录。? 是一个占位符，它会被 id 替换。通过这种方式，你可以动态地传递查询条件。

	.Delete(Article{})
		Delete(Article{}): 这个方法是 GORM 的删除操作，它会根据前面的 Where 条件，删除满足条件的记录。这里传入 Article{} 是一个空的 Article 结构体实例，它表示要删除的是 Article 表中的记录。
		注意：这里传入的是结构体的类型，而不是具体的记录。这是 GORM 中的一种写法，它会自动根据 Where 条件删除 Article 表中匹配的记录。
	*/
	db.Where("id = ?", id).Delete(Article{})

	return true
}

// 这里的 BeforeCreate 是 GORM 的 Hook 方法，当 article 结构体（Article）的实例即将被创建（写入数据库）时，这个方法会被自动调用
// 当 Article 结构体被保存到数据库之前，自动给 CreatedOn 赋值当前时间。
func (article *Article) BeforeCreate(scope *gorm.Scope) error {
	//用于 设置数据库列的值
	// "CreatedOn"：数据库表中的 CreatedOn 字段（通常是存储记录创建时间）。
	// time.Now().Unix()：获取当前时间的 Unix 时间戳（秒级）
	// 
	scope.SetColumn("CreatedOn", time.Now().Unix())

	// 返回 nil 表示执行成功，不会阻止数据插入
	// 如果返回 error，则 GORM 会中断创建操作，例如： return errors.New("创建文章失败")
	return nil
}

func ( article*Article) BeforeUpdate(scope *gorm.Scope) error {
	scope.SetColumn("ModifiedOn", time.Now().Unix())

	return nil
}

// 硬删除要使用 Unscoped()，这是 GORM 的约定
// 硬删除所有文章
func CleanAllArticle() bool {
	db.Unscoped().Where("deleted_on != ? ", 0).Delete(&Article{})

	return true
}
