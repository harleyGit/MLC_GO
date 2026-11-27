package hgerrors

const (
	HGMediaFileNotFound    Code = 2001001
	HGMediaFileTooLarge    Code = 2001002
	HGMediaTypeUnsupported Code = 2001003
	HGMediaUploadFailed    Code = 2001004
)

var hgMedia = []Definition{
	{HGMediaFileNotFound, "MEDIA_FILE_NOT_FOUND", "文件不存在", 404, false},
	{HGMediaFileTooLarge, "MEDIA_FILE_TOO_LARGE", "文件超出大小限制", 413, false},
	{HGMediaTypeUnsupported, "MEDIA_TYPE_UNSUPPORTED", "不支持的媒体类型", 415, false},
	{HGMediaUploadFailed, "MEDIA_UPLOAD_FAILED", "上传失败", 503, true},
}
