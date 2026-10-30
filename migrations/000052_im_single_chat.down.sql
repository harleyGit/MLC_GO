-- 仅适用于尚无业务数据的隔离环境；生产删除私信、设备回执和媒体索引需单独评审。
DROP TABLE im_media;
DROP TABLE im_device_delivery;
DROP TABLE im_device;
DROP TABLE im_message;
DROP TABLE im_conversation_member;
DROP TABLE im_conversation;
