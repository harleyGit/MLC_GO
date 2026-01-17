####
 # @Author: GangHuang harleysor@qq.com
 # @Date: 2026-01-15 21:18:14
 # @LastEditors: GangHuang harleysor@qq.com
 # @LastEditTime: 2026-01-15 21:18:21
 # @FilePath: /MLC_GO/scripts/db01.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE!
### 

#!/bin/bash
set -e

### ==== 读取环境变量 =====
export $(grep -v '^#' ../.env | xargs)

#TODO: 可以使用 \ 进行换行吗？？ 
MYSQL_CMD="mysql -h$MYSQL_HOST -P$MYSQL_PORT -u$MYSQL_USER -p$MYSQL_PASSWORD"
### ======================


### ==== 1. 检查MySQL服务 ====
check_mysql() {
    if ! mysqladmin ping \
    -h"$MYSQL_HOST" \
    -P"$MYSQL_PORT" \ 
    -u"$MYSQL_USER" \
    -p"$MYSQL_PASSWORD" \
    --silent; then
        echo "[INFO] MySQL 未启动，启动中...."
        brew services start mysql
        sleep 5
    fi
}
### ======================  

### ==== 2. 初始化数据库 ====
run_sql() {
    SQL_FILE=$1 
    if [ -z "$SQL_FILE" ]; then
        echo "[ERROR] 请输入 SQL 文件路径"
        exit 1
    sql_file
    FilePath=$(realpath "$SQL_FILE")    
    if [ ! -f "$SQL_FILE" ]; then
        echo "[ERROR] SQL 文件不存在: $SQL_FILE"
        exit 1
    fi
    echo "[INFO] 执行 SQL 文件: $SQL_FILE"
    $MYSQL_CMD "$MYSQL_DB" < "$SQL_FILE"
    echo "[INFO] SQL 文件执行完成: $SQL_FILE"
}
### ======================

### ==== 主流程 ====
mysql_cmd() {
    mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" \
    -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" 
}   
### ======================

mysql_shell() {
    echo "[INFO] 进入 MySQL Shell..."
    $MYSQL_CMD "$MYSQL_DB"
}
### ======================

### ==== 主流程 ====
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

    echo "[INFO] 执行 SQL 文件: $SQL_FILE"
    $MYSQL_CMD "$MYSQL_DB" < "$SQL_FILE"
    echo "[INFO] SQL 文件执行完成: $SQL_FILE"
}
### ======================

### ==== 命令分发 ====
case "$1" in
    init)
        check_mysql
        run_sql "../migrations/000init.sql"
        run_sql "../migrations/user_sql/000hg_crate_user.sql"
        ;;
    run_sql)
        check_mysql
        SQL_FILE=$2
        run_sql "$SQL_FILE"
        ;;  
    shell)
        check_mysql
        echo "[INFO] 进入 MySQL Shell..."
        $MYSQL_CMD "$MYSQL_DB"
        ;;
    *)
        echo "用法: $0 {init|shell}"
        echo "  init      初始化数据库"
        echo "  shell     进入 MySQL 交互模式"
        echo "  run_sql   运行指定的 SQL 文件"
        echo "示例: $0 init"
        echo "示例: $0 shell"
        echo "示例: $0 run_sql ../migrations/000init.sql"
        exit 1
        ;;
esac    
