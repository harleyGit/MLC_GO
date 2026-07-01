/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 11:43:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-01 21:04:24
 * @FilePath: /MLC_GO/internal/pkg/utils/hg_random_num.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UtilsPackage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	mathRand "math/rand"
	"os"
	"strings"
	"time"
)

// generateSeid 生成一个类似 Bilibili 的 seid（19~20 位数字字符串）
// 格式：时间戳(10位) + 随机6位 + 微秒后3位（可选）
func GenerateSeid() string {
	now := time.Now().Unix()
	// 生成 6 位随机数 [100000, 999999]
	randNum, _ := rand.Int(rand.Reader, big.NewInt(900000))
	seid := fmt.Sprintf("%d%06d", now, randNum.Int64()+100000)
	return seid
}

// Crockford Base32 字母表（避免 0/O, I/L 混淆）
const base32Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

const ulidEncodedLen = 26

// 构建多语言字符池（精选常用、可读、非生僻）
var charPool = []rune(
	// 中文（常用单字名）
	"伟芳娜敏静丽强磊洋艳婷杰娟涛明超浩宇欣彤涵睿萱怡然晨雪华红勇燕鹏丹" +
		// 日文（平假名 + 常见汉字，避免片假名太“机械”）
		"あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほ" +
		"山田中村佐藤鈴木高橋渡辺伊藤山本小林" +
		// 韩文（常用韩文字母组合，选完整音节）
		"김이박최정강조윤장임한서전홍유황진신권오" +
		// 英文（大小写，避免数字和符号，更像名字）
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz",
)

/* 生成随机验证码 */
func GenerazteVerifyCode() string {
	max := big.NewInt(1000000)
	n, _ := rand.Int(rand.Reader, max)
	return n.Text(10)
}

func GenerateRandomNum(n int) string {
	bytes := make([]byte, n) // byte切片,长度为n
	rand.Read(bytes)
	for i := range bytes { //遍历切片
		bytes[i] = '0' + bytes[i]%10 //取余然后转换为数字字符
	}
	return string(bytes) //转换为字符串返回
}

func GenerateRandomSalt() string {
	bytes := make([]byte, 16) // byte切片,长度为n
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateULID 生成 26 位 ULID 字符串。
// 前 10 位来自毫秒时间戳，后 16 位来自 80 bit 加密随机数；整体按时间大体有序，适合作为业务 ID 后缀。
func GenerateULID() string {
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		panic("failed to generate ulid entropy")
	}

	var id [ulidEncodedLen]byte
	encodeULIDTime(id[:10], uint64(time.Now().UnixMilli()))
	encodeULIDEntropy(id[10:], entropy)
	return string(id[:])
}

// GenerateBusinessID 生成固定格式的业务 ID：PREFIX_ + 26 位 ULID，例如 ROL_01JZ4M9T5P4P4CH7B4Y4QXAK8B。
func GenerateBusinessID(prefix string) string {
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	return prefix + "_" + GenerateULID()
}

// GenerateRoleID 生成角色业务 ID。
func GenerateRoleID() string {
	return GenerateBusinessID("ROL")
}

// GenerateUserIDWithRegion 生成用户业务 ID：HGUSR_ + 地域码 + 26 位 ULID。
func GenerateUserIDWithRegion(region string) string {
	region = normalizeRegion(region)
	if region == "" {
		region = "TW"
	}
	return "HGUSR_" + region + "_" + GenerateULID()
}

// CurrentRegion 根据当前运行环境推断地域码。
// 优先级：HG_REGION/REGION/AWS_REGION 环境变量 > TZ 时区环境变量 > time.Local > TW 兜底。
func CurrentRegion() string {
	for _, key := range []string{"HG_REGION", "REGION", "AWS_REGION"} {
		if region := normalizeRegion(os.Getenv(key)); region != "" {
			return region
		}
	}
	if region := regionFromTimezone(os.Getenv("TZ")); region != "" {
		return region
	}
	return regionFromTimezone(time.Local.String())
}

func encodeULIDTime(dst []byte, timestamp uint64) {
	for i := 9; i >= 0; i-- {
		dst[i] = base32Alphabet[timestamp&0x1F]
		timestamp >>= 5
	}
}

func encodeULIDEntropy(dst []byte, entropy [10]byte) {
	value := new(big.Int).SetBytes(entropy[:])
	mask := big.NewInt(31)
	for i := 15; i >= 0; i-- {
		dst[i] = base32Alphabet[new(big.Int).And(value, mask).Int64()]
		value.Rsh(value, 5)
	}
}

// generate16CharID 生成 16 位 Base32 随机字符串
func generate16CharID() string {
	// 16 字符 × 5 bits = 80 bits → 需要 10 字节随机数
	randBytes := make([]byte, 10)
	_, err := rand.Read(randBytes)
	if err != nil {
		panic("failed to generate random bytes")
	}

	// 将 10 字节（80 位）转为 16 个 Base32 字符
	id := make([]byte, 16)
	for i := 0; i < 16; i++ {
		// 每次取 5 位（从高位开始）
		// 手动位运算提取 5-bit chunk
		bitPos := i * 5
		byteIndex := bitPos / 8
		bitOffset := bitPos % 8

		// 从 randBytes 中取 2 个字节拼成 16 位，再右移取 5 位
		var val uint16
		if byteIndex+1 < len(randBytes) {
			val = (uint16(randBytes[byteIndex]) << 8) | uint16(randBytes[byteIndex+1])
		} else {
			val = uint16(randBytes[byteIndex]) << 8
		}
		shift := 16 - (bitOffset + 5)
		chunk := (val >> shift) & 0x1F // 取低 5 位
		id[i] = base32Alphabet[chunk]
	}
	return string(id)
}

// formatWithHyphens 每 4 位加一个 '-'
func formatWithHyphens(s string) string {
	if len(s) != 16 {
		panic("input must be 16 characters")
	}
	return s[0:4] + "-" + s[4:8] + "-" + s[8:12] + "-" + s[12:16]
}

/* 生成用户ID */
func GenerateUserID() string {
	return GenerateUserIDWithRegion(CurrentRegion())
}

func normalizeRegion(region string) string {
	region = strings.ToUpper(strings.TrimSpace(region))
	region = strings.ReplaceAll(region, "-", "_")
	if strings.HasPrefix(region, "AWS_") {
		region = strings.TrimPrefix(region, "AWS_")
	}
	if strings.HasPrefix(region, "AP_") || strings.HasPrefix(region, "US_") || strings.HasPrefix(region, "EU_") {
		return regionFromCloudRegion(region)
	}
	if len(region) > 8 {
		return ""
	}
	for _, r := range region {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return region
}

func regionFromCloudRegion(region string) string {
	switch {
	case strings.Contains(region, "AP_EAST_1"):
		return "HK"
	case strings.Contains(region, "AP_NORTHEAST_1"):
		return "JP"
	case strings.Contains(region, "AP_SOUTHEAST_1"):
		return "SG"
	case strings.HasPrefix(region, "US_"):
		return "US"
	case strings.HasPrefix(region, "EU_"):
		return "EU"
	default:
		return ""
	}
}

func regionFromTimezone(zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" || zone == "Local" {
		return "TW"
	}
	switch zone {
	case "Asia/Taipei":
		return "TW"
	case "Asia/Hong_Kong":
		return "HK"
	case "Asia/Shanghai", "Asia/Chongqing", "Asia/Urumqi":
		return "CN"
	case "Asia/Tokyo":
		return "JP"
	case "Asia/Seoul":
		return "KR"
	case "Asia/Singapore":
		return "SG"
	}
	if strings.HasPrefix(zone, "America/") || strings.HasPrefix(zone, "US/") {
		return "US"
	}
	if strings.HasPrefix(zone, "Europe/") {
		return "EU"
	}
	return "TW"
}

// uniqueRandomRunes 从 pool 中随机抽取 n 个不重复的 rune
func uniqueRandomRunes(pool []rune, n int) []rune {
	if n > len(pool) {
		n = len(pool)
	}
	// 打乱切片（Fisher-Yates shuffle 简化版）
	shuffled := make([]rune, len(pool))
	copy(shuffled, pool)
	mathRand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled[:n]
}

// GenerateMultilingualName 生成 1～6 位、无重复、多语言混合的名字
func GenerateMultilingualName() string {
	length := mathRand.Intn(6) + 1 // 1 ～ 6
	runes := uniqueRandomRunes(charPool, length)
	return string(runes)
}
func isEnglish(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

/* 生成包含英语的用户名 */
func GenerateMultilingualNameWithNonEnglish() string {
	for attempt := 0; attempt < 10; attempt++ {
		length := mathRand.Intn(6) + 1
		runes := uniqueRandomRunes(charPool, length)
		// 检查是否至少有一个非英文字符
		hasNonEng := false
		for _, r := range runes {
			if !isEnglish(r) {
				hasNonEng = true
				break
			}
		}
		if hasNonEng {
			return string(runes)
		}
	}
	// fallback: 直接加一个中文
	return "李" + GenerateMultilingualName()[:5]
}

// func main() {
//     fmt.Println("生成 10 个多语言名字：")
//     for i := 0; i < 10; i++ {
//         name := GenerateMultilingualName()
//         fmt.Printf("%d. %s (长度: %d)\n", i+1, name, len([]rune(name)))
//     }
// }
