package platform

import (
	"strings"
	"testing"
	"time"
)

const hgDetailFixture = `{"code":0,"data":{"bvid":"BV1GJ411x7h7","aid":123,"title":" 测试视频 ","desc":" 简介 ","pic":"http://i0.hdslb.com/cover.jpg","tname":"科技","duration":12,"pubdate":1700000000,"owner":{"mid":42,"name":" 作者 ","face":"https://i0.hdslb.com/face.jpg"},"stat":{"view":100,"like":8,"coin":2,"favorite":5,"share":1,"reply":6,"danmaku":3},"pages":[{"cid":7,"page":1,"part":" 第一集 ","duration":12}]}}`

func TestHGCleanDetailSeparatesReplyAndDanmaku(t *testing.T) {
	d, err := HGCleanBilibiliDetail([]byte(hgDetailFixture), "BV1GJ411x7h7", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "测试视频" || d.Description != "简介" || d.Stats.Reply != 6 || d.Stats.Danmaku != 3 || d.Parts[0].CID != "7" || d.CoverURL != "https://i0.hdslb.com/cover.jpg" {
		t.Fatalf("清洗结果错误: %+v", d)
	}
}

func TestHGCleanDetailRejectsInvalidSnapshots(t *testing.T) {
	for _, test := range []struct{ name, old, next string }{
		{"负计数", `"view":100`, `"view":-1`},
		{"缺失计数", `"view":100,`, ``},
		{"空计数", `"view":100`, `"view":null`},
		{"字符串计数", `"view":100`, `"view":"100"`},
		{"错误身份", `"bvid":"BV1GJ411x7h7"`, `"bvid":"BV1GJ411x7h8"`},
		{"空标题", `"title":" 测试视频 "`, `"title":" "`},
		{"错误分P", `"page":1`, `"page":2`},
		{"危险图片", `https://i0.hdslb.com/face.jpg`, `https://localhost/face.jpg`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := HGCleanBilibiliDetail([]byte(strings.Replace(hgDetailFixture, test.old, test.next, 1)), "BV1GJ411x7h7", time.Now()); err == nil {
				t.Fatal("应拒绝无效快照")
			}
		})
	}
}

func TestHGCleanDetailAllowsUpstreamStatMetadata(t *testing.T) {
	hgBody := strings.Replace(hgDetailFixture, `"view":100`, `"evaluation":"","argue_msg":"","view":100`, 1)
	if _, hgErr := HGCleanBilibiliDetail([]byte(hgBody), "BV1GJ411x7h7", time.Now()); hgErr != nil {
		t.Fatalf("上游额外统计元数据不应导致清洗失败: %v", hgErr)
	}
}
