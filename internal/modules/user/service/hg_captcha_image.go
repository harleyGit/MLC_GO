/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-31
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-06-05 23:07:19
 * @FilePath: /MLC_GO/internal/modules/user/service/hg_captcha_image.go
 * @Description: 点选验证码图片生成工具
 *
 * ┌─────────────────────────────────────────────────────────────────────┐
 * │  整体流程（一次验证码请求的完整链路）                                  │
 * │                                                                     │
 * │  1. generateCaptchaID()        → 生成唯一 ID，用于关联验证码答案      │
 * │  2. generateRandomChars(3)     → 从字符池随机抽取 3 个字符            │
 * │  3. generateRandomPoints(3,w,h)→ 在画布上生成 3 个不重叠的坐标        │
 * │  4. generateCaptchaImage()     → 把字符渲染到坐标位置，输出 Base64    │
 * │  5. 存储：ID → {chars, points} 写入 Redis，后续验证时比对             │
 * │  6. 返回：{id, imageBase64, chars(明文)} 给前端                      │
 * └─────────────────────────────────────────────────────────────────────┘
 */
package UserServicePackage

import (
	"bytes"
	_ "embed" // 仅执行 embed 注入，不直接调用 embed 包 API
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/image/font"          // 字体渲染核心：Face 接口、Drawer、Metrics
	"golang.org/x/image/font/opentype"  // OpenType/TrueType 字体解析器
	"golang.org/x/image/math/fixed"     // 定点数运算（字体渲染坐标系用 fixed.Int26_6）
)

// ============================================================
// 字体资源（编译期嵌入）
// ============================================================

//go:embed assets/fonts/NotoSansSC-Regular.ttf
var captchaFontBytes []byte
// captchaFontBytes 是编译期通过 //go:embed 嵌入的 NotoSansSC 字体文件字节。
// 为什么用 embed？
//   - 避免运行时依赖外部文件路径，部署时无需关心字体文件位置
//   - 字体文件随二进制一起分发，零额外 I/O
//   - NotoSansSC 支持中文 + 英文 + 数字，覆盖验证码所有字符

// ============================================================
// 全局共享变量（只读 + 线程安全）
// ============================================================

var (
	// captchaFontOnce / captchaFont / captchaFontErr 三者配合，保证 TTF 解析只执行一次。
	// sync.Once 语义：即使 N 个 goroutine 同时调用 Do，也只有一个执行闭包，其余阻塞等待完成。
	captchaFontOnce sync.Once
	captchaFont     *opentype.Font // 解析后的字体对象（只读，所有 goroutine 共享）
	captchaFontErr  error          // 解析失败时的错误（只写一次，之后不再变化）

	// captchaFacePool 缓存 font.Face 对象，避免每次生成验证码都 NewFace。
	//
	// sync.Pool 语义：
	//   - Get() 优先从池中取已有对象（零分配），池空时调用 New 创建新对象
	//   - Put() 用完归还，下次 Get 可复用
	//   - GC 时池中对象可能被回收，不影响正确性（下次 Get 会重新创建）
	//
	// 为什么能用 Pool？
	//   - font.Face 内部的光栅化器在单次 drawChar 调用间无残留状态
	//   - 同一时刻一个 face 只被一个 goroutine 使用（Get → 用 → Put 是串行的）
	//   - 因此 Pool 中的 face 可以安全地被不同请求复用
	//
	// FaceOptions 参数说明：
	//   - Size: 26pt —— 验证码字符的渲染字号，太小难以辨认，太大放不下 280×100 画布
	//   - DPI: 72 —— 屏幕标准 DPI（每英寸72点），用于 pt→px 换算
	//             实际像素 = Size × DPI / 72 = 26 × 72 / 72 = 26px
	//   - Hinting: HintingFull —— 完全微调，让字形轮廓对齐像素网格，
	//             渲染更清晰锐利（验证码需要清晰可读，不能模糊）
	captchaFacePool = sync.Pool{
		New: func() any {
			face, err := opentype.NewFace(captchaFont, &opentype.FaceOptions{
				Size:    26,
				DPI:     72,
				Hinting: font.HintingFull,
			})
			if err != nil {
				// sync.Pool.New 签名是 func() any，不能返回 (T, error)
				// 所以用 errFacePoolItem 包装错误，调用方 Get 后做类型断言检查
				return errFacePoolItem{err: err}
			}
			return face
		},
	}
)

// errFacePoolItem 是 sync.Pool.New 创建失败时的占位对象。
type errFacePoolItem struct{ err error }

// ============================================================
// 初始化
// ============================================================

func init() {
	// 用当前时间作为随机数种子，保证每次启动后随机序列不同。
	// 注意：Go 1.20+ 的全局 rand 已自动随机化种子，这里保留是为了兼容旧版本。
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

// ============================================================
// 验证码 ID 生成
// ============================================================

// generateCaptchaID 生成唯一的验证码 ID。
//
// 格式：毫秒时间戳(13位) + 6位随机数，共19位数字字符串。
// 示例："1717612345678012345"
//
// 为什么这样设计？
//   - 时间戳保证单调递增，天然避免同一毫秒内的 ID 冲突（配合随机数进一步降低碰撞概率）
//   - 6位随机数 = 100万种可能，同一毫秒内两个请求生成相同 ID 的概率 = 百万分之一
//   - 用于 Redis key 的一部分，如 auth:captcha:{id}，需要全局唯一
func generateCaptchaID() string {
	return fmt.Sprintf("%d%06d", time.Now().UnixMilli(), rand.Intn(1000000))
}

// ============================================================
// 随机字符生成
// ============================================================

// generateRandomChars 生成指定数量的随机字符（数字、字母、汉字混合）。
//
// 参数：
//   - count: 需要生成的字符数量，通常是 3（点选验证码一般 3 个字符）
//
// 返回值：
//   - []string: 长度为 count 的字符串切片，每个元素是一个字符（可能是多字节汉字）
//
// 安全性分析：
//   - 数字 10 个 + 字母 50 个（去掉了易混淆的 O/0/I/l 等）+ 汉字 300 个 = 360 个字符
//   - 3 个字符的组合数 = 360³ = 46,656,000（约 4665 万种）
//   - 再加上 3 个点选坐标（画布 280×100 = 28000 像素），总空间 ≈ 4665万 × 28000³ ≈ 10^20 量级
//   - 即使攻击者知道字符池和画布尺寸，穷举成本也极高
func generateRandomChars(count int) []string {
	// 字符池：数字、大写字母、小写字母、常用汉字。
	// 排除了易混淆字符：0/O、1/I/l、2/Z、5/S、8/B（数字和字母去重）
	digits := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	letters := []string{
		"A", "B", "C", "D", "E", "F", "G", "H", "J", "K", "L", "M",
		"N", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
		"a", "b", "c", "d", "e", "f", "g", "h", "j", "k", "m", "n",
		"p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	}
	// 汉字池：300个常用汉字，覆盖日常高频用字
	// 安全性：300选3 = 2700万种组合，加上数字62选3 ≈ 23.8万，总空间 ≈ 6.4万亿
	// 即使攻击者知道字符池，穷举成本也极高；配合点选坐标，暴力破解几乎不可能
	chinese := []string{
		// 天文地理
		"日", "月", "星", "天", "地", "山", "水", "火", "风", "雨",
		"雪", "云", "雷", "电", "海", "河", "湖", "江", "泉", "石",
		"土", "沙", "尘", "雾", "霜", "露", "冰", "潮", "浪", "波",
		// 四季时令
		"春", "夏", "秋", "冬", "晨", "暮", "昼", "夜", "年", "岁",
		"朝", "夕", "午", "时", "分", "秒", "刻", "期", "节", "令",
		// 动物植物
		"花", "草", "树", "木", "林", "森", "竹", "松", "柏", "梅",
		"兰", "菊", "荷", "桃", "李", "杏", "梨", "柳", "桐", "桂",
		"鸟", "鱼", "虫", "马", "牛", "羊", "犬", "猫", "鸡", "鸭",
		"龙", "凤", "鹤", "鹰", "燕", "雁", "雀", "蝶", "蜂", "蝉",
		// 人物称谓
		"人", "男", "女", "父", "母", "子", "女", "兄", "弟", "姐",
		"妹", "友", "师", "生", "王", "民", "主", "客", "宾", "君",
		// 身体部位
		"头", "手", "足", "目", "耳", "口", "鼻", "心", "面", "身",
		// 动作行为
		"走", "跑", "飞", "跳", "行", "立", "坐", "卧", "看", "听",
		"说", "读", "写", "画", "唱", "舞", "打", "拿", "放", "开",
		"关", "来", "去", "出", "入", "上", "下", "左", "右", "中",
		"前", "后", "东", "西", "南", "北", "进", "退", "回", "转",
		// 状态性质
		"大", "小", "多", "少", "长", "短", "高", "低", "远", "近",
		"快", "慢", "冷", "热", "新", "旧", "好", "坏", "美", "丑",
		"明", "暗", "深", "浅", "轻", "重", "软", "硬", "甜", "苦",
		"红", "黄", "蓝", "绿", "白", "黑", "紫", "粉", "灰", "金",
		// 生活用品
		"门", "窗", "灯", "车", "船", "桥", "路", "房", "楼", "塔",
		"桌", "椅", "床", "碗", "杯", "盘", "刀", "剑", "琴", "棋",
		"书", "画", "纸", "笔", "墨", "砚", "镜", "钟", "旗", "衣",
		// 社会文化
		"国", "家", "城", "村", "街", "市", "厂", "店", "园", "宫",
		"军", "兵", "将", "战", "胜", "败", "和", "平", "安", "乐",
		"文", "武", "道", "德", "仁", "义", "礼", "智", "信", "忠",
		"学", "问", "思", "想", "知", "见", "言", "语", "词", "诗",
		// 抽象概念
		"爱", "恨", "情", "愁", "梦", "意", "志", "力", "能", "气",
		"灵", "魂", "命", "生", "死", "始", "终", "因", "果", "理",
		"真", "善", "正", "直", "诚", "敬", "勇", "敢", "刚", "柔",
		// 数量方位
		"一", "二", "三", "四", "五", "六", "七", "八", "九", "十",
		"百", "千", "万", "亿", "半", "双", "对", "群", "队", "阵",
		// 饮食相关
		"米", "面", "肉", "菜", "果", "茶", "酒", "盐", "糖", "油",
		"饭", "粥", "汤", "饼", "豆", "麦", "谷", "粮", "鲜", "香",
	}

	// 合并所有字符到一个切片：digits(10) + letters(50) + chinese(300) = 360 个
	allChars := append(append(digits, letters...), chinese...)
	result := make([]string, count)

	// 从 360 个字符中随机抽取 count 个（允许重复）
	for i := 0; i < count; i++ {
		result[i] = allChars[rand.Intn(len(allChars))]
	}

	return result
}

// ============================================================
// 随机坐标生成
// ============================================================

// generateRandomPoints 生成指定数量的随机坐标点。
//
// 参数：
//   - count: 需要生成的坐标数量（与字符数一致，通常是 3）
//   - width: 画布宽度（像素），当前是 280
//   - height: 画布高度（像素），当前是 100
//
// 返回值：
//   - []ClickCaptchaPoint: 不重叠的随机坐标数组
//
// 算法：拒绝采样（Rejection Sampling）
//   - 每次生成一个随机坐标，检查与所有已有点的距离
//   - 如果距离 < minSpacing(50px)，则重新生成，直到满足条件
//   - 保证任意两个字符之间至少 50px 间距，避免字符重叠导致无法点选
//
// 为什么用 padding=30？
//   - 字符渲染时以坐标为中心，26pt 字号约 26px 宽
//   - 30px 边距保证字符不会被画布边缘裁切
func generateRandomPoints(count, width, height int) []ClickCaptchaPoint {
	points := make([]ClickCaptchaPoint, count)
	padding := 30    // 边距，避免字符太靠近边缘
	minSpacing := 50 // 字符之间的最小间距（欧几里得距离）

	for i := 0; i < count; i++ {
		valid := false
		for !valid {
			// 在 [padding, width-padding) × [padding, height-padding) 范围内随机取点
			points[i] = ClickCaptchaPoint{
				X: padding + rand.Intn(width-2*padding),
				Y: padding + rand.Intn(height-2*padding),
			}
			valid = true
			// 检查与所有已有点的距离，确保不重叠
			for j := 0; j < i; j++ {
				dx := points[i].X - points[j].X
				dy := points[i].Y - points[j].Y
				// 欧几里得距离 < minSpacing 则不合格，重新生成
				if math.Sqrt(float64(dx*dx+dy*dy)) < float64(minSpacing) {
					valid = false
					break
				}
			}
		}
	}

	return points
}

// ============================================================
// 验证码图片生成（核心函数）
// ============================================================

// generateCaptchaImage 生成验证码图片并返回 Base64 编码。
//
// 参数：
//   - chars: 要绘制的字符数组，如 ["风", "A", "3"]
//   - points: 每个字符对应的坐标，与 chars 一一对应
//
// 返回值：
//   - string: PNG 图片的 Base64 编码，可直接用于 <img src="data:image/png;base64,...">
//   - error: 字体加载失败或 PNG 编码失败时返回
//
// 生成步骤：
//   1. 从 sync.Pool 获取 font.Face（字体渲染面）
//   2. 创建 280×100 的 RGBA 画布
//   3. 填充浅灰色背景
//   4. 绘制干扰线（5 条随机线段，干扰 OCR 识别）
//   5. 绘制干扰点（50 个随机色点，进一步干扰）
//   6. 在每个坐标点绘制对应字符
//   7. 编码为 PNG → Base64
func generateCaptchaImage(chars []string, points []ClickCaptchaPoint) (string, error) {
	width := 280  // 画布宽度（像素）
	height := 100 // 画布高度（像素）

	// 从 sync.Pool 获取一个 font.Face，用完后归还
	face, err := newCaptchaFontFace()
	if err != nil {
		return "", err
	}
	defer putCaptchaFontFace(face) // 确保用完归还到 Pool，供下次复用

	// 创建 RGBA 图片（每个像素 4 字节：R/G/B/A）
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 填充背景色（浅灰色 R:245 G:245 B:245）
	// 浅灰色背景让深色字符更清晰，同时避免纯白被 OCR 直接跳过
	bgColor := color.RGBA{R: 245, G: 245, B: 245, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// 先画干扰线和干扰点（背景层），再画字符（前景层）
	// 这样字符会覆盖在干扰之上，保证可读性
	drawInterferenceLines(img, width, height) // 5 条随机灰色线段
	drawInterferenceDots(img, width, height)  // 50 个随机色点

	// 在每个坐标点绘制对应字符
	for i, char := range chars {
		if i < len(points) {
			drawChar(img, face, char, points[i].X, points[i].Y)
		}
	}

	// 将图片编码为 PNG 格式
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}

	// 转换为 Base64 字符串，前端可直接用 <img src="data:image/png;base64,{result}">
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ============================================================
// 字体 Face 管理（sync.Pool 缓存）
// ============================================================

// newCaptchaFontFace 从 sync.Pool 获取一个 font.Face。
//
// 整体流程：
//   1. sync.Once 保证 TTF 字节 → *opentype.Font 只解析一次（全局复用）
//   2. 从 sync.Pool 获取 font.Face（池空时自动 New）
//
// 为什么用 sync.Once？
//   - opentype.Parse 是 CPU 密集操作（解析 TTF 字体表：cmap/glyf/head/hhea 等）
//   - 字体数据是只读的，解析结果 *opentype.Font 可以被所有 goroutine 安全共享
//   - sync.Once 保证即使并发调用，也只执行一次解析，后续调用直接返回缓存
//
// 为什么用 sync.Pool？
//   - font.Face 内部持有光栅化器（rasterizer），不是线程安全的，不能全局共享
//   - 但同一个 face 在单次 drawChar 调用间无残留状态，用完可以安全复用
//   - sync.Pool 让 face 在不同请求间复用，避免每次 NewFace 的 CPU 开销
//   - GC 时池中对象可被自动回收，不影响正确性
//
// 使用方式：
//   face, err := newCaptchaFontFace()
//   if err != nil { return err }
//   defer putCaptchaFontFace(face) // 用完必须归还！
//
// 返回值：
//   - font.Face: 可直接传给 font.Drawer.DrawString 绘制文字
//   - error: 字体解析或 face 创建失败时返回
func newCaptchaFontFace() (font.Face, error) {
	// 第一步：sync.Once 保证 TTF 字体文件只被解析一次
	// captchaFontBytes 来自 //go:embed 编译期嵌入的 NotoSansSC-Regular.ttf
	// opentype.Parse 会解析 TTF 文件的 cmap（字符→字形映射表）、glyf（字形轮廓）等
	// 解析结果 *opentype.Font 是只读的，存入包级变量 captchaFont，所有 goroutine 共享
	captchaFontOnce.Do(func() {
		captchaFont, captchaFontErr = opentype.Parse(captchaFontBytes)
	})
	if captchaFontErr != nil {
		return nil, fmt.Errorf("parse captcha font: %w", captchaFontErr)
	}

	// 第二步：从 sync.Pool 获取 font.Face
	// 池中有现成的就直接复用（零分配），池空则调用 Pool.New 创建新的
	item := captchaFacePool.Get()
	// 检查是否是创建失败的占位对象
	if ei, ok := item.(errFacePoolItem); ok {
		return nil, fmt.Errorf("create captcha font face: %w", ei.err)
	}
	return item.(font.Face), nil
}

// putCaptchaFontFace 归还 font.Face 到 sync.Pool，供下次 Get 复用。
// 必须与 newCaptchaFontFace 配对使用，否则 Pool 中的对象会泄漏。
func putCaptchaFontFace(face font.Face) {
	captchaFacePool.Put(face)
}

// ============================================================
// 干扰线绘制（抗 OCR）
// ============================================================

// drawInterferenceLines 绘制 5 条随机干扰线。
//
// 目的：干扰 OCR 图像识别，让机器难以自动提取字符轮廓。
// 算法：Bresenham 线段绘制算法的简化版（逐像素插值）。
//
// 参数：
//   - img: 目标 RGBA 图片
//   - width, height: 画布尺寸
//
// 为什么用灰色系？
//   - 浅灰色（160-200）不会盖住深色字符（50-150），保证人眼可读
//   - 但对 OCR 来说，灰色线段会打断字符的连续轮廓，增加识别难度
func drawInterferenceLines(img *image.RGBA, width, height int) {
	lineColors := []color.RGBA{
		{R: 200, G: 200, B: 200, A: 255}, // 浅灰
		{R: 180, G: 180, B: 180, A: 255}, // 中灰
		{R: 160, G: 160, B: 160, A: 255}, // 深灰
	}

	for i := 0; i < 5; i++ {
		c := lineColors[rand.Intn(len(lineColors))]
		// 随机生成线段的两个端点（可能超出画布，后面有边界检查）
		x1, y1 := rand.Intn(width), rand.Intn(height)
		x2, y2 := rand.Intn(width), rand.Intn(height)

		// 计算线段需要绘制的像素数（取 x/y 方向的较大值）
		// 这样即使线段很陡（dy >> dx），也能保证每个像素行都有一个点
		steps := int(math.Max(math.Abs(float64(x2-x1)), math.Abs(float64(y2-y1))))
		if steps == 0 {
			continue // 两个端点重合，跳过
		}

		// 逐像素插值绘制线段
		// t 从 0.0 到 1.0，线性插值出线段上每个像素的坐标
		for step := 0; step <= steps; step++ {
			t := float64(step) / float64(steps)
			x := int(float64(x1) + t*float64(x2-x1))
			y := int(float64(y1) + t*float64(y2-y1))

			// 边界检查：只绘制画布范围内的像素
			if x >= 0 && x < width && y >= 0 && y < height {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

// ============================================================
// 干扰点绘制（抗 OCR）
// ============================================================

// drawInterferenceDots 绘制 50 个随机干扰点。
//
// 目的：与干扰线配合，进一步增加图像噪声，干扰 OCR 二值化处理。
// 为什么用浅色（180-255）？
//   - 浅色点在浅灰色背景上不太明显，人眼几乎无感
//   - 但对 OCR 的二值化阈值判断会产生干扰（像素灰度值在阈值附近跳变）
func drawInterferenceDots(img *image.RGBA, width, height int) {
	for i := 0; i < 50; i++ {
		x := rand.Intn(width)
		y := rand.Intn(height)
		// R/G/B 各在 [180, 255) 范围内随机，生成浅色系干扰点
		c := color.RGBA{
			R: uint8(180 + rand.Intn(75)),
			G: uint8(180 + rand.Intn(75)),
			B: uint8(180 + rand.Intn(75)),
			A: 255,
		}
		img.SetRGBA(x, y, c)
	}
}

// ============================================================
// 字符绘制（核心渲染逻辑）
// ============================================================

// drawChar 在指定坐标位置绘制一个字符。
//
// 原理：
//   1. 用 font.Drawer 把字符光栅化到一个临时的 Alpha 遮罩（mask）上
//   2. 遮罩中非零像素 = 字符轮廓，零像素 = 背景
//   3. 把遮罩中非零像素映射到目标图片的对应位置，填充随机深色
//
// 为什么用 Alpha 遮罩而不是直接画到 img 上？
//   - font.Drawer.DrawString 直接画到 dst 时，会用 Src 颜色覆盖 dst 像素
//   - 这样无法实现"以坐标为中心绘制"（需要偏移 Dot 位置）
//   - 用 Alpha 遮罩可以精确控制每个像素的绘制位置和颜色
//
// 参数：
//   - img: 目标 RGBA 图片
//   - face: 字体渲染面（从 sync.Pool 获取）
//   - char: 要绘制的单个字符（可能是多字节汉字）
//   - x, y: 字符中心坐标（drawChar 会自动居中）
func drawChar(img *image.RGBA, face font.Face, char string, x, y int) {
	// 随机选择一个深色系颜色绘制字符
	// 深色（50-150）在浅灰背景（245）上对比度高，人眼清晰可读
	// 但对 OCR 来说，随机颜色增加了颜色通道分析的难度
	charColors := []color.RGBA{
		{R: 50, G: 50, B: 150, A: 255},   // 深蓝
		{R: 150, G: 50, B: 50, A: 255},   // 深红
		{R: 50, G: 120, B: 50, A: 255},   // 深绿
		{R: 100, G: 50, B: 120, A: 255},  // 深紫
		{R: 50, G: 100, B: 150, A: 255},  // 深青
	}
	c := charColors[rand.Intn(len(charColors))]

	// ---- 第一步：测量字符尺寸 ----
	metrics := face.Metrics()
	// MeasureString 返回字符的水平宽度（fixed.Int26_6 定点数），Ceil() 向上取整到像素
	textWidth := font.MeasureString(face, char).Ceil()
	// Ascent（基线以上高度）+ Descent（基线以下高度）= 字符总高度
	textHeight := (metrics.Ascent + metrics.Descent).Ceil()
	if textWidth <= 0 || textHeight <= 0 {
		return // 空字符或测量失败，跳过
	}

	// ---- 第二步：创建 Alpha 遮罩并绘制字符 ----
	// Alpha 遮罩：单通道图片，每个像素只有透明度（0=透明，255=不透明）
	mask := image.NewAlpha(image.Rect(0, 0, textWidth, textHeight))
	d := &font.Drawer{
		Dst:  mask,            // 绘制目标：Alpha 遮罩
		Src:  image.White,     // 绘制源颜色：白色（Alpha 遮罩中只用 Alpha 通道）
		Face: face,            // 字体渲染面
		Dot:  fixed.P(0, metrics.Ascent.Ceil()), // 绘制起点：x=0, y=Ascent（基线位置）
		// 为什么 y = Ascent？
		//   - 字体坐标系以基线（baseline）为 y=0
		//   - Ascent 是基线到字符顶部的距离，所以 y=Ascent 让字符顶部对齐遮罩顶部
	}
	d.DrawString(char) // 把字符光栅化到 mask 上

	// ---- 第三步：把遮罩中的字符像素映射到目标图片 ----
	// 计算字符在目标图片上的起始位置（以 x,y 为中心居中）
	startX := x - textWidth/2
	startY := y - textHeight/2
	bounds := img.Bounds()

	// 遍历遮罩的每个像素
	for py := 0; py < textHeight; py++ {
		for px := 0; px < textWidth; px++ {
			// Alpha = 0 表示透明（字符轮廓外的空白区域），跳过
			if mask.AlphaAt(px, py).A == 0 {
				continue
			}
			// 计算该像素在目标图片上的坐标
			dx := startX + px
			dy := startY + py
			// 边界检查：只绘制画布范围内的像素（防止字符超出画布导致越界）
			if dx >= bounds.Min.X && dx < bounds.Max.X && dy >= bounds.Min.Y && dy < bounds.Max.Y {
				img.SetRGBA(dx, dy, c) // 用随机深色填充字符像素
			}
		}
	}
}
