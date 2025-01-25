/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-23 14:38:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-01-25 13:40:03
 * @FilePath: /MLC_GO/TestNotes/TestCrawlerProgram/TestCrawlerBaidu/test_crawler_baidu.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE.
 */
package main

import (
	"fmt"
	"net/http"
	"os"

	// "io"
	"bufio"

	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly"
)

func main() {
	// 根据http://news.baidu.com开始抓取页面结果
	//testCrawlerBaidu()

	// 访问指定网址
	//testCrawlerAppointWebpage("https://news.baidu.com")

	// 存储爬虫文件
	// testCrawlerAppointWebpageAndSaveTxt("https://news.baidu.com")
	
	// 把爬虫程序设置成Web服务
	testCrawlerAppointWebpageAndSaveTxtV2()
}

// 把爬虫程序设置成Web服务
func testCrawlerAppointWebpageAndSaveTxtV2() {
	// 初始化引擎
	engine := gin.Default()
	// 注册一个路由和处理函数
	engine.Any("/", WebRoot)
	// 绑定端口
	engine.Run(":9200")
}
// 处理路由函数
func WebRoot(context *gin.Context){
	// 调用 testCrawlerAppointWebpageAndSaveTxt()函数
	testCrawlerAppointWebpageAndSaveTxt("https://news.baidu.com")
	// 设置网页上的文本内容
	context.String(http.StatusOK, "已经把http://news.baidu.com 的网页内容全部抓取下来\n 请查看当前项目目录下的news.txt文本文件！")
}

// 存储爬虫文件
func testCrawlerAppointWebpageAndSaveTxt(urls string) {
	// 创建 Collector 对象
	c := colly.NewCollector()
	/* 
	* 是否抓取指定链接的网页内容
	* 初始设置为不抓取指定链接的网页内容
	 */
	visited := false
	// 使用 Collector 对象抓取 URL
	c.OnResponse(func (r *colly.Response)  {
		if !visited {
			visited = true
			r.Request.Visit("/get?q=2")
		}
	})

	// 文件的创建和持续写入
	filename := "news.txt"	// 文件名
	var f *os.File
	var fileErr error 
	if testCheckFileExist(filename) {	// 如果文件存在
		f, fileErr = os.OpenFile(filename, os.O_APPEND | os.O_WRONLY, 0666)	// 打开文件
		if fileErr != nil {
			fmt.Printf("❌ 打开文件 news.txt 失败：%v\n", fileErr)
		}
	} else {
		f,_ = os.Create(filename)	// 创建文件
		fmt.Println("🍎 新建文件 news.txt")

	}
	defer f.Close()	// 关闭文件
	write := bufio.NewWriter(f)
	defer write.Flush()

	// 对指定链接网页内容进行处理
	c.OnHTML("a[href]", func (e *colly.HTMLElement)  {
		href := e.Text	// 获取指定链接的网页内容
		write.WriteString("🍒 "+href+"\n")
		fmt.Println("🍎 网页内容：",href)	// 打印指定链接的网页内容
	})


	c.Visit(urls)	//访问指定链接
}
/* 检测文件是否存在
* fileName 文件名
 */
func testCheckFileExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

/// 抓取指定连接的网页内容 urls: 指定链接
func testCrawlerAppointWebpage(urls string) {
	// 创建 Collector 对象
	c := colly.NewCollector()
	/* 
	* 是否抓取指定链接的网页内容
	* 初始设置为不抓取指定链接的网页内容
	 */
	visited := false
	// 使用 Collector 对象抓取 URL
	c.OnResponse(func (r *colly.Response)  {
		if !visited {
			visited = true
			r.Request.Visit("/get?q=2")
		}
	})

	// 对指定链接网页内容进行处理
	c.OnHTML("a[href]", func (e *colly.HTMLElement)  {
		href := e.Text	// 获取指定链接的网页内容
		fmt.Println("🍎 网页内容：",href)	// 打印指定链接的网页内容
	})
	c.Visit(urls)	//访问指定链接
}

// / 爬虫百度新闻网页
func testCrawlerBaidu() {
	c := colly.NewCollector(
		// colly.AllowedDomains("news.baidu.com"),
		colly.UserAgent("Opera/9.80(Windows NT 6.1; U; zh-cn) Presto/2.9.168 Version/11.50"))
	// 发出请求时向 Collector 对象附加的回调函数
	c.OnRequest(func(r *colly.Request) {
		// 设置请求头
		r.Headers.Set("Host", "baidu.com")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Accept", "*/*")
		r.Headers.Set("Origin", "")
		r.Headers.Set("Referer", "http://www.baidu.com")
		r.Headers.Set("Accept-Encoding", "gzip, deflate")
		r.Headers.Set("Accept-Language", "zh-CN, zh;q=0.9")
		fmt.Println("正在访问：", r.URL)
	})

	// 处理HTML中的文档标题
	c.OnHTML("title", func(e *colly.HTMLElement) {
		fmt.Println("文档标题：", e.Text)
	})

	// 处理HTML中的文档内容
	c.OnHTML("body", func(e *colly.HTMLElement) {
		e.ForEach(".hotnews a", func(i int, el *colly.HTMLElement) {
			band := el.Attr("href")
			title := el.Text
			fmt.Printf("文档内容 %d: %s\n%s\n", i+1, title, band)
		})
	})

	// 获取收到的响应内容的数量
	c.OnResponse(func(r *colly.Response) {
		fmt.Println("收到响应内容的数量： ", r.StatusCode)
	})

	// 限制 visit 的线程数， visit 可以同时运行多个
	c.Limit(&colly.LimitRule{
		Parallelism: 2,
	})

	c.Visit("http://news.baidu.com")
}
