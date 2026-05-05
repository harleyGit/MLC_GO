package UserServicePackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	"MLC_GO/internal/pkg/logHG"
	HGUploadPackage "MLC_GO/internal/pkg/upload"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"sync"
)

// region 错误定义

var (
	ErrAvatarUploadFailed = errors.New("头像上传失败")
	ErrAvatarNotFound     = errors.New("头像不存在")
)

// endregion

// region 响应结构

// AvatarUploadResult 头像上传结果。
type AvatarUploadResult struct {
	AvatarURL string `json:"avatarUrl"` // 头像访问 URL
	FileName  string `json:"fileName"`  // 文件名
	IsNew     bool   `json:"isNew"`     // 是否新上传
}

// endregion

// region 头像服务

// AvatarService 头像服务，支持百万级并发。
type AvatarService struct {
	userSvc  *UserService
	uploader *HGUploadPackage.Uploader

	// 并发控制：每个用户同时只能上传一次
	userLocks sync.Map
}

// NewAvatarService 创建头像服务。
func NewAvatarService(userSvc *UserService) *AvatarService {
	return &AvatarService{
		userSvc:  userSvc,
		uploader: HGUploadPackage.NewDefaultUploader(),
	}
}

// getUserLock 获取用户级别的锁，防止同一用户并发上传。
func (s *AvatarService) getUserLock(userID string) *sync.Mutex {
	val, _ := s.userLocks.LoadOrStore(userID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// endregion

// region 业务方法

// UploadAvatarFromBytes 从二进制数据上传用户头像。
// 流程：
//  1. 检查用户当前头像，如果存在且文件存在则直接返回
//  2. 上传新头像到文件系统
//  3. 更新 users 表的 avatar_url 字段
//  4. 删除旧头像文件
func (s *AvatarService) UploadAvatarFromBytes(ctx context.Context, userID string, imageData []byte, ext string) (*AvatarUploadResult, error) {
	// 1. 获取用户锁，防止并发上传
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	// 2. 查询用户当前头像
	userDTO, err := s.userSvc.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 3. 如果已有头像，检查文件是否存在
	if userDTO.AvatarURL != nil && *userDTO.AvatarURL != "" {
		oldFilePath := "." + *userDTO.AvatarURL
		if HGUploadPackage.CheckFileExists(oldFilePath) {
			// 文件已存在，直接返回
			return &AvatarUploadResult{
				AvatarURL: *userDTO.AvatarURL,
				FileName:  s.getFileNameFromURL(*userDTO.AvatarURL),
				IsNew:     false,
			}, nil
		}
	}

	// 4. 上传新头像（从二进制数据）
	result, err := s.uploader.UploadFromBytes(imageData, "user", ext)
	if err != nil {
		logHG.ErrFInfo("上传头像失败: %v", err)
		return nil, ErrAvatarUploadFailed
	}

	// 5. 更新 users 表的 avatar_url
	avatarURL := result.FileURL
	updateDTO := &UserDtoPackage.HGUpdateUserProfileReqDTO{
		AvatarURL: &avatarURL,
	}
	if _, err := s.userSvc.UpdateProfile(ctx, userID, updateDTO); err != nil {
		return nil, fmt.Errorf("更新头像URL失败: %w", err)
	}

	// 6. 删除旧头像文件
	if userDTO.AvatarURL != nil && *userDTO.AvatarURL != "" {
		oldFilePath := "." + *userDTO.AvatarURL
		if err := HGUploadPackage.DeleteFile(oldFilePath); err != nil {
			logHG.ErrFInfo("删除旧头像失败: %v", err)
			// 不影响主流程，只记录日志
		}
	}

	return &AvatarUploadResult{
		AvatarURL: avatarURL,
		FileName:  result.FileName,
		IsNew:     true,
	}, nil
}

// UploadAvatar 从 multipart 文件上传用户头像（兼容旧方式）。
func (s *AvatarService) UploadAvatar(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*AvatarUploadResult, error) {
	// 读取文件内容
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer src.Close()

	// 读取全部内容
	imageData := make([]byte, fileHeader.Size)
	if _, err := src.Read(imageData); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 获取文件扩展名
	ext := HGUploadPackage.GetFileExt(fileHeader.Filename)

	return s.UploadAvatarFromBytes(ctx, userID, imageData, ext)
}

// GetAvatarURL 获取用户头像 URL。
func (s *AvatarService) GetAvatarURL(ctx context.Context, userID string) (string, error) {
	userDTO, err := s.userSvc.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if userDTO.AvatarURL == nil {
		return "", nil
	}
	return *userDTO.AvatarURL, nil
}

// endregion

// region 辅助方法

// getFileNameFromURL 从 URL 提取文件名。
func (s *AvatarService) getFileNameFromURL(url string) string {
	if url == "" {
		return ""
	}
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			return url[i+1:]
		}
	}
	return url
}

// endregion
