/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-05 13:08:13
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 22:06:30
 * @FilePath: /MLC_GO/TestNotes/CrawlerPracticeExample/main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

/**
*  爬取豆瓣电影 TOP250: https://eddycjy.com/posts/go/crawler/2018-03-21-douban-top250/
*  爬取汽车之家 二手车产品库: https://eddycjy.com/posts/go/crawler/2018-04-01-cars/
*	了解一下Golang的市场行情 项目地址https://github.com/go-crawler/lagou_jobs
 */
package main

import (
	"MLC_GO/TestNotes/CrawlerPracticeExample/model"
	"MLC_GO/TestNotes/CrawlerPracticeExample/parse"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var BaseUrl = "https://movie.douban.com/top250"

func main(){
	Start()
	
	showMovieData()

	defer model.CloseDB()
}

func showMovieData() {

	var movies []parse.DoubanMovie

	// 查询所有电影数据
	if err := model.DB.Find(&movies).Error; err != nil {
		logging.ErrInfo("查询失败: ", err)
	}

	// 打印查询结果
	for _, movie := range movies {
		logging.DebugInfo("desc: %d, Title: %s, Year: %d\n", movie.Desc, movie.Title, movie.Year)
	}
}

// 新增数据
func Add(movies []parse.DoubanMovie) {
	logging.DebugInfo("豆瓣插入数据movies：", movies)

	for index, movie := range movies {
		// 将 movie 插入数据库。
		// Error：如果 Create 过程中发生错误，则 Error 变量会存储错误信息
		if err := model.DB.Create(&movie).Error; err != nil {
			logging.DebugInfo("db.Create index: ", index, "err: ", err)
		}

		logging.DebugInfo("豆瓣插入数据：", movie)
	}
}

func Start() {
	var movies []parse.DoubanMovie

	pages := parse.GetPages(BaseUrl)
	for _, page := range pages {
		doc, err := goquery.NewDocument(strings.Join([]string{BaseUrl, page.Url}, ""))
		if err != nil {
			logging.ErrInfo("main.go-Start: ",err)
		}

		movies = append(movies, parse.ParseMovies(doc)...)
	}
	Add(movies)
}

