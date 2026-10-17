package VideoUploadServicePackage

import (
	"context"
	"errors"
	"strings"
)

// HGValidateExternalPart 校验已入库快照的CID，禁止客户端指定任意媒体URL。
func (s *Service) HGValidateExternalPart(ctx context.Context, id string) (string, string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 4 || parts[0] != "external" || parts[1] != "bilibili" || len(id) > 64 {
		return "", "", errors.New("外部分P标识无效")
	}
	item, found, err := s.GetVideoListItem(ctx, strings.Join(parts[:3], ":"))
	if err != nil {
		return "", "", err
	}
	if found && item.HGExternalDetail != nil {
		for _, part := range item.HGExternalDetail.Parts {
			if part.CID == parts[3] {
				return parts[2], parts[3], nil
			}
		}
	}
	return "", "", errors.New("分P尚未入库")
}
