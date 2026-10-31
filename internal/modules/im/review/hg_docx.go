package review

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

// hgInspectArchive 限制 ZIP/Office 包目录与膨胀大小，拒绝宏；它不能替代病毒扫描。
func hgInspectArchive(path string, size int64, mime string) (int, int, int64, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer reader.Close()
	if size <= 0 || len(reader.File) == 0 || len(reader.File) > 2000 {
		return 0, 0, 0, fmt.Errorf("Office 文件目录超限")
	}
	var total uint64
	contentType, document := false, false
	for _, entry := range reader.File {
		name := strings.ToLower(entry.Name)
		if strings.HasPrefix(entry.Name, "/") || strings.Contains(entry.Name, "\\") || strings.Contains(entry.Name, "../") ||
			strings.HasSuffix(name, ".bin") || strings.Contains(name, "vbaproject") || strings.Contains(name, "externallinks") ||
			entry.FileInfo().Mode()&os.ModeSymlink != 0 || entry.Flags&1 != 0 ||
			entry.Method != zip.Store && entry.Method != zip.Deflate {
			return 0, 0, 0, fmt.Errorf("Office 文件存在不允许的内容")
		}
		total += entry.UncompressedSize64
		if entry.UncompressedSize64 > 32<<20 || total > 256<<20 {
			return 0, 0, 0, fmt.Errorf("Office 文件解压体积超限")
		}
		contentType = contentType || entry.Name == "[Content_Types].xml"
		document = document || entry.Name == "word/document.xml"
		if strings.HasSuffix(name, ".rels") && mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
			if entry.UncompressedSize64 > 1<<20 {
				return 0, 0, 0, fmt.Errorf("Office 关系文件超限")
			}
			body, err := entry.Open()
			if err != nil {
				return 0, 0, 0, err
			}
			decoder := xml.NewDecoder(io.LimitReader(body, 1<<20))
			for {
				token, parseErr := decoder.Token()
				if parseErr == io.EOF {
					break
				}
				if parseErr != nil {
					body.Close()
					return 0, 0, 0, fmt.Errorf("Office 关系文件不合法")
				}
				if start, ok := token.(xml.StartElement); ok {
					for _, attr := range start.Attr {
						if attr.Name.Local == "TargetMode" && strings.EqualFold(attr.Value, "External") {
							body.Close()
							return 0, 0, 0, fmt.Errorf("Office 外部关系被拒绝")
						}
					}
				}
			}
			body.Close()
		}
	}
	if mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" && (!contentType || !document) {
		return 0, 0, 0, fmt.Errorf("Office 文件结构不完整")
	}
	return 0, 0, 0, nil
}
