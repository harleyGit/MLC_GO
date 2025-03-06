/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-05 11:34:29
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 18:01:45
 * @FilePath: /MLC_GO/TestNotes/CrawlerPracticeExample/docs/parse/douban.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package parse

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type DoubanMovie struct {
	ID       uint   `gorm:"primaryKey"`
	Title    string `gorm:"type:varchar(30);default:'';comment:'标题'"`
	Subtitle string `gorm:"type:varchar(20);default:'';comment:'副标题'"`
	Other    string `gorm:"type:varchar(20);default:'';comment:'其他'"`
	Desc     string `gorm:"type:varchar(30);default:'';comment:'简述'"`
	Year     string   `gorm:"type:varchar(30);default:'';comment:'年份'"`
	Area     string `gorm:"type:varchar(20);default:'';comment:'地区'"`
	Tag      string `gorm:"type:varchar(20);default:'';comment:'标签'"`
	Star     string   `gorm:"type:varchar(30);default:'';comment:'star'"`
	Comment  string   `gorm:"type:varchar(30);default:'';comment:'评分'"`
	Quote    string `gorm:"type:varchar(30);default:'';comment:'引用'"`
}

// TableName 指定表名，防止 GORM 默认加 "s"
func (DoubanMovie) TableName() string {
	return "sp_douban_movie"
}

type Page struct {
	Page int
	Url string
}

// 获取分页
func GetPages(url string) []Page {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		logging.ErrInfo(err)
	}

	return ParsePages(doc)
}

// 分析分页
// 获取所有分叶
func ParsePages(doc *goquery.Document) (pages []Page) {
	pages = append(pages, Page{Page: 1, Url: ""})
	doc.Find("#content > div > div.article > div.paginator > a").Each(func(i int, s *goquery.Selection) {
		page, _ := strconv.Atoi(s.Text())
		url, _ := s.Attr("href")
		
		pages = append(pages, Page{
			Page: page,
			Url: url,
		})
	})

	return pages
}

// 分析豆瓣电影信息
func ParseMovies(doc *goquery.Document) (movies []DoubanMovie) {
	doc.Find("#content > div > div.article > ol > li").Each(func(i int, s *goquery.Selection) {
		title := s.Find(".hd s pan").Eq(0).Text()

		subtitle := s.Find(".hd a span").Eq(1).Text()
		subtitle = strings.TrimLeft(subtitle, " / ")

		other := s.Find(".hd a span").Eq(2).Text()
		other = strings.TrimLeft(other, " / ")

		desc := strings.TrimSpace(s.Find(".bd p").Eq(0).Text())
		DescInfo := strings.Split(desc, "\n")
		desc = DescInfo[0]

		movieDesc := strings.Split(DescInfo[1], "/")
		year := strings.TrimSpace(movieDesc[0])
		area := strings.TrimSpace(movieDesc[1])
		tag := strings.TrimSpace(movieDesc[2])

		star := s.Find(".bd .star .rating_num").Text()

		comment := strings.TrimSpace(s.Find(".bd .star span").Eq(3).Text())
		compile := regexp.MustCompile("[0-9]")
		comment = strings.Join(compile.FindAllString(comment, -1), "")

		quote := s.Find(".quote .inq").Text()

		movie := DoubanMovie{
			Title: title,
			Subtitle: subtitle,
			Other: other,
			Desc: desc,
			Year: year,
			Area: area,
			Tag: tag,
			Star: star,
			Comment: comment,
			Quote: quote,
		}

		logging.DebugInfo("i: ", i, ", movie: ", movie)
		
		movies = append(movies, movie)
	})

	return movies
}