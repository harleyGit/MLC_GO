# ###
 # @Author: GangHuang harleysor@qq.com
 # @Date: 2026-01-15 20:28:27
 # @LastEditors: GangHuang harleysor@qq.com
 # @LastEditTime: 2026-01-15 20:36:33
 # @FilePath: /MLC_GO/scripts/db.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 #
 # 功能：核心脚本，用于初始化数据库和运行迁移脚本，支持【任意路径SQL + 交互执行】统一脚本
### 

#!/bin/bash
set -e

### ==== 基础配置（后期可以改为环境变量）=====
MYSQL_USER="root"
MYSQL_PASSWORD="" #"hh109" #Intel电脑sql没有密码，M2Pro密码是hh109
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
MYSQL_DB="HG_MLC_DB"
### ========================================

### ==== 公共方法 ====
mysql_cmd() {
    mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" \
    -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" 
    #-D "$DATABASE_NAME" -e "$1"
}
### =================

### ==== 1. 检查MySQL服务 ====
check_mysql() {
    local MYSQLADMIN_ARGS=(
        -h"$MYSQL_HOST"
        -P"$MYSQL_PORT"
        -u"$MYSQL_USER"
    )

    # 仅当密码非空时才添加 -p 参数
    if [ -n "$MYSQL_PASSWORD" ]; then
        MYSQLADMIN_ARGS+=(-p"$MYSQL_PASSWORD")
    fi

    if ! mysqladmin ping "${MYSQLADMIN_ARGS[@]}" --silent; then
        echo "[INFO] MySQL 未启动，启动中...."
        brew services start mysql
        sleep 5
    fi
}

check_mysql00() {
    if ! mysqladmin ping \
    -h"$MYSQL_HOST" \
    -P"$MYSQL_PORT" \
    -u"$MYSQL_USER" \
    -p"$MYSQL_PASSWORD" \
    --silent; then


        echo "[INFO] MySQL 未启动，启动中...."
        brew services start mysql
        sleep 5
        #echo "MySQL服务未运行或无法连接，请检查配置。"
        #exit 1
    fi
    #echo "MySQL服务连接成功。"
}
### ======================


### ===== 2. 初始化数据库 =====
init_db() {
    echo "[INFO] 初始化数据库"
    mysql_cmd < ../migrations/init.sql
}
### ======================

### ==== 3. 执行指定 SQL 文件  ====
run_sql() {
    SQL_FILE=$1

    if [ -z "$SQL_FILE" ]; then
        echo "[ERROR] 请输入 SQL 文件路径"
        exit 1
    fi

    if [ ! -f "$SQL_FILE" ]; then
        echo "[ERROR] SQL 文件不存在: $SQL_FILE"
        exit 1
    fi

    echo "[INFO] 运行 SQL 文件: $SQL_FILE"
    mysql_cmd "$MYSQL_DB"< "$SQL_FILE"
}
### ======================


### ==== 4.进入MySQL 交互模式 ====
mysql_shell() {
    echo "[INFO] 进入 MySQL Shell..."
    mysql_cmd "$MYSQL_DB"
}
### ======================


### ==== 命令分发 ====
case "$1" in
    init_db)
        check_mysql
        init_db
        #echo "[INFO] 初始化数据库: $MYSQL_DB"
        #mysql_cmd -e "CREATE DATABASE IF NOT EXISTS $MYSQL_DB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;"
        ;;
    run)
        check_mysql
        run_sql "$2"
        ;;
    shell)
        check_mysql
        mysql_shell
        ;;
    *)
        #echo "用法: $0 {init_db|run_sql <sql_file>|mysql_shell}"
        echo "用法："
        echo "chmod +x ./scripts/db.sh          # 赋予执行权限【若是有就不需要执行】"
        echo "  ./scripts/db.sh init_db                # 初始化数据库"
        echo "  ./scripts/db.sh run path/to/xxx.sql<sql_file>         # 运行指定的 SQL 文件"
        echo "  ./scripts/db.sh shell                   # 进入 MySQL 交互模式【也就是终端】"
        exit 1
        ;;
esac
### ======================