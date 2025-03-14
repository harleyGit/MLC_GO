/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-12 17:11:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-14 19:02:12
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/service/tag_service/tag.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package tag_service

import (
	"MLC_GO/TestNotes/GenPracticeExample/models"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/export"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/file"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/gredis"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/service/cache_service"
	"encoding/json"
	"strconv"
	"time"

	"github.com/tealeg/xlsx"
)

type Tag struct {
	ID         int
	Name       string
	CreatedBy  string
	ModifiedBy string
	State      int

	PageNum  int
	PageSize int
}

func (t *Tag)ExistByID() (bool, error) {
	return models.ExistTagByID(t.ID)
}

func (t *Tag) GetAll() ([]models.Tag, error) {
	var (
		tags, cacheTags []models.Tag
	)

	cache := cache_service.Tag{
		State: t.State,

		PageNum: t.PageNum,
		PageSize: t.PageSize,
	}
	key := cache.GetTagsKey()
	if gredis.Exists(key) {
		data, err := gredis.Get(key)
		if err != nil {
			logging.ErrInfo(err)
		} else {
			json.Unmarshal(data, &cacheTags)
			return cacheTags, nil
		}
	}

	tags, err := models.GetTags(t.PageNum, t.PageSize, t.getMaps())
	if err != nil {
		return nil, err
	}

	gredis.Set(key, tags, 3600)
	return tags, nil
}

// 导出Excle文件
func (t *Tag) Export() (string, error) {
	tags, err := t.GetAll()
	if err != nil {
		return "", err
	}
	// 创建一个新的 Excel 文件对象。这个对象将用于保存后续添加的工作表和数据。
	xlsFile := xlsx.NewFile()
	// 在 Excel 文件中添加一个名为 "标签信息" 的工作表。
	// sheet：返回的是一个工作表对象，可以用来添加行、单元格等数据
	sheet, err := xlsFile.AddSheet("标签信息")
	if err != nil {
		return "", err
	}

	titles := []string{"ID", "名称", "创建人", "创建时间", "修改人", "修改时间"}
	// 定义一个字符串切片 titles，包含需要显示在 Excel 文件第一行的各个标题。每个标题代表一个列名，如 "ID"、"名称" 等
	row := sheet.AddRow()

	var cell *xlsx.Cell
	for _, title := range titles {
		cell = row.AddCell()
		cell.Value = title
	}

	for _, v := range tags {
		values := []string {
			strconv.Itoa(v.ID),
			v.Name,
			v.CreatedBy,
			strconv.Itoa(v.CreatedOn),
			v.ModifiedBy,
			strconv.Itoa(v.ModifiedOn),
		}
		// sheet.AddRow() 方法在当前工作表中添加一行，并返回这个新行的引用。后续将会在这行中添加各个标题单元格。
		row = sheet.AddRow()
		for _, value := range values {
			// row.AddCell()：在当前行中添加一个新的单元格，并返回该单元格的引用，赋值给 cell。
			cell = row.AddCell()
			// 将当前标题字符串赋值给单元格的 Value 属性，使得单元格显示对应的标题内容。
			cell.Value = value
		}
	}

	// strconv.Itoa:整数转换为它的字符串形式
	time := strconv.Itoa(int(time.Now().Unix()))
	filename := "tags-" + time + export.EXT

	dirFullPath := export.GetExcelFullPath()
	err = file.IsNotExistMkDir(dirFullPath)
	if err != nil {
		return "", err
	}

	err = xlsFile.Save(dirFullPath + filename)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func (t *Tag) getMaps() map[string]interface{} {
	maps := make(map[string]interface{})

	maps["deleted_on"] = 0

	if t.Name != "" {
		maps["name"] = t.Name
	}
	if t.State >= 0 {
		maps["state"] = t.State
	}

	return maps
}