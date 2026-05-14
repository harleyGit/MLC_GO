-- ============================================
-- 创建数据库：HG_MLC_DB
-- ============================================

-- CREATE DATABASE:
-- 用于创建一个新的数据库
--
-- IF NOT EXISTS:
-- 表示“如果不存在则创建”
-- 防止数据库已经存在时执行报错
--
-- HG_MLC_DB:
-- 数据库名称
--
-- DEFAULT CHARACTER SET utf8mb4:
-- 指定数据库默认字符集为 utf8mb4
--
-- utf8mb4 是 MySQL 推荐使用的 UTF-8 字符集：
--   1. 支持中文、英文、日文、韩文等多语言
--   2. 支持 Emoji 表情 😀😂🚀
--   3. 支持完整 Unicode 字符
--
-- 注意：
-- MySQL 里的 utf8 并不是真正完整 UTF-8，
-- 它最多只支持 3 字节字符，
-- 无法存储 emoji。
--
-- COLLATE utf8mb4_general_ci:
-- 指定排序规则（校对规则）
--
-- 含义：
--   utf8mb4     -> 对应字符集
--   general     -> 通用排序规则
--   ci          -> case insensitive（忽略大小写）
--
-- 即：
--   'Tom' = 'tom'
--
-- 常见排序规则：
--   utf8mb4_general_ci      通用，性能较好
--   utf8mb4_unicode_ci      Unicode 标准排序，更准确
--   utf8mb4_0900_ai_ci      MySQL8 推荐使用
CREATE DATABASE IF NOT EXISTS HG_MLC_DB
DEFAULT CHARACTER SET utf8mb4
COLLATE utf8mb4_general_ci;



-- ============================================
-- 切换当前使用的数据库
-- ============================================
USE HG_MLC_DB;