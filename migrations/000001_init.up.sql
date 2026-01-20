-- 创建主数据库，使用 utf8mb4 支持完整 Unicode（包括 emoji 😀）
CREATE DATABASE IF NOT EXISTS HG_MLC_DB 
DEFAULT CHARACTER SET utf8mb4
COLLATE utf8mb4_general_ci;

-- 切换到刚创建的数据库（注意名称一致性！）
USE HG_MLC_DB;