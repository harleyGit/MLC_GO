package review

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// hgBoundedOutput 拒绝媒资内嵌大量流造成探测输出无界增长。
type hgBoundedOutput struct {
	bytes.Buffer
}

func (w *hgBoundedOutput) Write(p []byte) (int, error) {
	if w.Len()+len(p) > 64<<10 {
		return 0, fmt.Errorf("ffprobe 输出超限")
	}
	return w.Buffer.Write(p)
}

type hgProbeResult struct {
	Streams []struct {
		CodecType string `json:"codec_type"` // video/audio。
		CodecName string `json:"codec_name"` // ffprobe 实际解码器名称。
		Width     int    `json:"width"`      // 像素宽度。
		Height    int    `json:"height"`     // 像素高度。
	} `json:"streams"` // 文件中的流，最多接受一个视觉流和一个音频流。
	Format struct {
		Duration string `json:"duration"` // 秒数字符串，图像和文档可以为空。
	} `json:"format"` // ffprobe 探测的容器信息。
}

func hgInspect(ctx context.Context, ffprobe, path, mime string, size int64) (int, int, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()
	var head [16]byte
	if _, err := io.ReadFull(file, head[:]); err != nil {
		return 0, 0, 0, fmt.Errorf("媒体头不足")
	}
	if !hgMagic(mime, head[:]) {
		return 0, 0, 0, fmt.Errorf("文件真实容器与 MIME 不一致")
	}
	if strings.HasPrefix(mime, "image/") && !hgImageBoundary(file, head[:], mime, size) {
		return 0, 0, 0, fmt.Errorf("图片结构或结尾不合法")
	}
	if mime == "application/zip" || strings.Contains(mime, "wordprocessingml") {
		return hgInspectArchive(path, size, mime)
	}
	if mime == "application/pdf" {
		return 0, 0, 0, nil
	}
	work, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(work, ffprobe, "-v", "error", "-show_entries", "stream=codec_type,codec_name,width,height:format=duration", "-of", "json", path)
	var output hgBoundedOutput
	cmd.Stdout = &output
	cmd.Stderr = &hgBoundedOutput{}
	if err := cmd.Run(); err != nil {
		return 0, 0, 0, fmt.Errorf("媒体探测失败")
	}
	var probe hgProbeResult
	if json.Unmarshal(output.Bytes(), &probe) != nil || len(probe.Streams) == 0 || len(probe.Streams) > 2 {
		return 0, 0, 0, fmt.Errorf("媒体流不合法")
	}
	visual, audio := 0, 0
	width, height := 0, 0
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			visual++
			width, height = stream.Width, stream.Height
			if width < 1 || width > 16384 || height < 1 || height > 16384 || !hgVideoCodec(mime, stream.CodecName) {
				return 0, 0, 0, fmt.Errorf("视频流不合法")
			}
		case "audio":
			audio++
			if !hgAudioCodec(mime, stream.CodecName) {
				return 0, 0, 0, fmt.Errorf("音频流不合法")
			}
		default:
			return 0, 0, 0, fmt.Errorf("媒体流包含未知类型")
		}
	}
	if strings.HasPrefix(mime, "image/") {
		if visual != 1 || audio != 0 {
			return 0, 0, 0, fmt.Errorf("图片流不合法")
		}
		return width, height, 0, nil
	}
	if strings.HasPrefix(mime, "video/") && visual != 1 || strings.HasPrefix(mime, "audio/") && (visual != 0 || audio != 1) {
		return 0, 0, 0, fmt.Errorf("媒体类型与实际流不一致")
	}
	seconds, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds >= float64(math.MaxInt64)/1000 || strings.HasPrefix(mime, "audio/") && seconds > 3600 {
		return 0, 0, 0, fmt.Errorf("媒体时长不合法")
	}
	duration := int64(math.Round(seconds * 1000))
	if duration < 1 || strings.HasPrefix(mime, "audio/") && duration > 3600000 {
		return 0, 0, 0, fmt.Errorf("媒体时长不合法")
	}
	return width, height, duration, nil
}

// hgImageBoundary 拒绝解码器忽略的尾随数据，避免附加的恶意载荷绕过图片容器扫描。
func hgImageBoundary(file *os.File, head []byte, mime string, size int64) bool {
	switch mime {
	case "image/jpeg":
		var end [2]byte
		_, err := file.ReadAt(end[:], size-2)
		return err == nil && end == [2]byte{0xff, 0xd9}
	case "image/webp":
		return int64(binary.LittleEndian.Uint32(head[4:8]))+8 == size
	case "image/png":
		offset := int64(8)
		for count := 0; count < 100000 && offset+12 <= size; count++ {
			var chunk [8]byte
			if _, err := file.ReadAt(chunk[:], offset); err != nil {
				return false
			}
			length := int64(binary.BigEndian.Uint32(chunk[:4]))
			if length > size-offset-12 {
				return false
			}
			offset += 12 + length
			if string(chunk[4:]) == "IEND" {
				return length == 0 && offset == size
			}
		}
	}
	return false
}

func hgMagic(mime string, head []byte) bool {
	switch mime {
	case "image/jpeg":
		return bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff})
	case "image/png":
		return bytes.HasPrefix(head, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	case "image/webp":
		return string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP"
	case "video/mp4", "audio/mp4":
		return string(head[4:8]) == "ftyp" && string(head[8:12]) != "qt  "
	case "video/quicktime":
		return string(head[4:8]) == "ftyp" && string(head[8:12]) == "qt  "
	case "audio/ogg":
		return string(head[:4]) == "OggS"
	case "audio/mpeg":
		return string(head[:3]) == "ID3" || head[0] == 0xff && head[1]&0xe0 == 0xe0
	case "audio/aac":
		return head[0] == 0xff && head[1]&0xf0 == 0xf0
	case "application/pdf":
		return string(head[:5]) == "%PDF-"
	case "application/zip", "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return bytes.HasPrefix(head, []byte{'P', 'K', 3, 4})
	default:
		return false
	}
}

func hgVideoCodec(mime, codec string) bool {
	switch mime {
	case "image/jpeg":
		return codec == "mjpeg"
	case "image/png":
		return codec == "png"
	case "image/webp":
		return codec == "webp"
	case "video/mp4", "video/quicktime":
		return codec == "h264" || codec == "hevc" || codec == "av1"
	default:
		return false
	}
}

func hgAudioCodec(mime, codec string) bool {
	switch mime {
	case "audio/mpeg":
		return codec == "mp3"
	case "audio/aac":
		return codec == "aac"
	case "audio/ogg":
		return codec == "opus" || codec == "vorbis"
	case "audio/mp4", "video/mp4", "video/quicktime":
		return codec == "aac" || codec == "alac"
	default:
		return false
	}
}
