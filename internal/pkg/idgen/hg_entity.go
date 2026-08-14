package idgen

// EntityType 标识业务 ID 的实体类型。业务代码应使用已注册常量，避免直接传字符前缀。
type EntityType uint8

const (
	// TypeUser 表示用户 ID，前缀为 U。
	TypeUser EntityType = iota + 1
	// TypeVideo 表示视频 ID，前缀为 V。
	TypeVideo
	// TypeComment 表示评论 ID，前缀为 C。
	TypeComment
	// TypeFollow 表示关注关系 ID，前缀为 O。
	TypeFollow
	// TypeLike 表示点赞 ID，前缀为 L。
	TypeLike
	// TypeFavorite 表示收藏 ID，前缀为 A。
	TypeFavorite
	// TypeCoin 表示投币 ID，前缀为 N。
	TypeCoin
	// TypeShare 表示分享 ID，前缀为 S。
	TypeShare
	// TypeMessage 表示消息 ID，前缀为 M。
	TypeMessage
	// TypeRoom 表示房间 ID，前缀为 R。
	TypeRoom
	// TypeDanmaku 表示弹幕 ID，前缀为 D。
	TypeDanmaku
	// TypeTag 表示标签 ID，前缀为 T。
	TypeTag
	// TypePlaylist 表示播放列表 ID，前缀为 P。
	TypePlaylist
	// TypeOrder 表示订单 ID，前缀为 X。
	TypeOrder
)

// HGEntityPrefix 返回业务类型的固定单字节前缀；未注册类型返回 ErrHGUnknownEntityType。
func HGEntityPrefix(entityType EntityType) (byte, error) {
	switch entityType {
	case TypeUser:
		return 'U', nil
	case TypeVideo:
		return 'V', nil
	case TypeComment:
		return 'C', nil
	case TypeFollow:
		return 'O', nil
	case TypeLike:
		return 'L', nil
	case TypeFavorite:
		return 'A', nil
	case TypeCoin:
		return 'N', nil
	case TypeShare:
		return 'S', nil
	case TypeMessage:
		return 'M', nil
	case TypeRoom:
		return 'R', nil
	case TypeDanmaku:
		return 'D', nil
	case TypeTag:
		return 'T', nil
	case TypePlaylist:
		return 'P', nil
	case TypeOrder:
		return 'X', nil
	default:
		return 0, ErrHGUnknownEntityType
	}
}

func hgEntityTypeFromPrefix(prefix byte) (EntityType, error) {
	switch prefix {
	case 'U':
		return TypeUser, nil
	case 'V':
		return TypeVideo, nil
	case 'C':
		return TypeComment, nil
	case 'O':
		return TypeFollow, nil
	case 'L':
		return TypeLike, nil
	case 'A':
		return TypeFavorite, nil
	case 'N':
		return TypeCoin, nil
	case 'S':
		return TypeShare, nil
	case 'M':
		return TypeMessage, nil
	case 'R':
		return TypeRoom, nil
	case 'D':
		return TypeDanmaku, nil
	case 'T':
		return TypeTag, nil
	case 'P':
		return TypePlaylist, nil
	case 'X':
		return TypeOrder, nil
	default:
		return 0, ErrHGUnknownEntityType
	}
}
