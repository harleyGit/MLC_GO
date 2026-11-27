// hg_error_catalog 从运行时字典离线导出 JSON 或迁移用 INSERT，只写标准输出。
package main

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	"MLC_GO/internal/response/hgerrors"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	format := flag.String("format", "json", "导出格式：json 或 seed")
	baseline := flag.String("baseline", "", "可选历史 JSON 快照；拒绝删除、改义或重用已发布错误")
	flag.Parse()
	if *baseline != "" {
		if err := hgCheckBaseline(*baseline); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := hgExport(os.Stdout, *format); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func hgCheckBaseline(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 1<<20 {
		return fmt.Errorf("历史错误快照超过 1 MiB 上限")
	}
	var defs []hgerrors.Definition
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&defs); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("历史错误快照包含多余或超限内容")
	}
	return hgerrors.HGValidateCompatibility(defs)
}

func hgExport(w io.Writer, format string) error {
	defs := hgerrors.HGDefinitions()
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(defs)
	case "seed":
		var b strings.Builder
		b.WriteString(q.HGErrorCodeCatalogInsertSQL)
		for i, d := range defs {
			if i > 0 {
				b.WriteString(",\n")
			}
			fmt.Fprintf(&b, q.HGErrorCodeCatalogRowSQL, d.Code, d.Key, d.Code.HGModule(), int(d.Code)/1000%100, d.HTTPStatus, strings.ReplaceAll(d.Message, "'", "''"), d.Retryable)
		}
		b.WriteString(";\n")
		_, err := io.WriteString(w, b.String())
		return err
	default:
		return fmt.Errorf("不支持导出格式 %q", format)
	}
}
