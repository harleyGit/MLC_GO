-- 创建主数据库，使用 utf8mb4 支持完整 Unicode（包括 emoji 😀）
CREATE DATABASE HG_MLC_DB DEFAULT CHARACTER SET utf8mb4;
-- 切换到刚创建的数据库（注意名称一致性！）
USE HG_MLC_DB;

CREATE TABLE Users(
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(128) UNIQUE,
    phone VARCHAR(32) UNIQUE,
    password_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
);