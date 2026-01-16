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

# 这是“shebang”行，告诉系统这个脚本要用 /bin/bash（Bash 解释器）来运行
# 就像 Python 脚本开头写 #!/usr/bin/env python3 一样，这是脚本的“身份证”，说明用什么程序来执行它。
#!/bin/bash
# 开启“严格模式”。只要脚本中任何一行命令返回非零退出码（即出错），整个脚本就立即停止，不再继续执行。
# 为什么重要？防止错误被忽略。比如连接数据库失败了，后面不该继续执行初始化。
set -e

### ==== 基础配置（后期可以改为环境变量）=====
# 定义变量，存储连接 MySQL 所需的信息
MYSQL_USER="root"
MYSQL_PASSWORD="hh109" # "" #Intel电脑sql没有密码，M2Pro密码是hh109
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
MYSQL_DB="HG_MLC_DB"
### ========================================

### ==== 公共方法 ====
# 作用： 定义一个函数（叫 mysql_cmd），它会拼接出连接 MySQL 的完整命令
mysql_cmd() {
    # mysql 是 MySQL 客户端命令
    # -h 后面跟主机（host）
    mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" \
    -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" 
    #-D "$DATABASE_NAME" -e "$1"
}
### =================

### ==== 1. 检查MySQL服务 ====
# 作用：检查 MySQL 是否正在运行。如果没运行，就用 brew 启动它（说明作者用的是 macOS + Homebrew）。
check_mysql() {
    local MYSQLADMIN_ARGS=(
        -h"$MYSQL_HOST"
        -P"$MYSQL_PORT"
        -u"$MYSQL_USER"
    )

    # 仅当密码非空时才添加 -p 参数
    # if ! ... ; then：如果 ping 失败（前面加 ! 表示“取反”），就执行里面的代码
    if [ -n "$MYSQL_PASSWORD" ]; then
        MYSQLADMIN_ARGS+=(-p"$MYSQL_PASSWORD")
    fi

    # mysqladmin ping：是 MySQL 自带的一个工具，用来“探测”数据库是否活着。如果能收到回复，说明服务正常。
    # --silent：不输出多余信息，只关心成功/失败。
    if ! mysqladmin ping "${MYSQLADMIN_ARGS[@]}" --silent; then
        echo "[INFO] MySQL 未启动，启动中...."
        # 用 Homebrew 启动 MySQL 服务（仅限 macOS 用户；Linux 用户可能是 systemctl start mysql）
        brew services start mysql
        # 暂停 5 秒，等 MySQL 完全启动好再继续
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
# 执行一个叫 migrations/init.sql 的 SQL 文件，用来创建表、插入初始数据等
init_db() {
    echo "[INFO] 初始化数据库"
    # 下面一行代码的作用相当于：用 mysql 连接数据库，并执行 init.sql 里的所有 SQL 语句
    # mysql_cmd 是前面定义的函数，返回一个连接命令
    # < migrations/init.sql：这是“输入重定向”，意思是“把 init.sql 文件的内容当作输入，喂给 mysql 命令”
    mysql_cmd < ../migrations/init.sql
}
### ======================

### ==== 3. 执行指定 SQL 文件  ====
run_sql() {
    # $1 是 Bash 中的第一个位置参数（即调用函数时传的第一个参数）。
    # 比如 run_sql abc.sql，那么 $1 就是 abc.sql，赋值给变量 SQL_FILE。
    SQL_FILE=$1

    #[ -z "$SQL_FILE" ]：判断字符串是否为空（-z 表示“zero length”）
    if [ -z "$SQL_FILE" ]; then
        echo "[ERROR] 请输入 SQL 文件路径"
        exit 1
    fi

    #判断文件是否存在且是普通文件（-f）
    if [ ! -f "$SQL_FILE" ]; then
        echo "[ERROR] SQL 文件不存在: $SQL_FILE"
        exit 1
    fi

    echo "[INFO] 运行 SQL 文件: $SQL_FILE"
    # 这里和 init_db 类似，但多了个数据库名：mysql_cmd "$MYSQL_DB"。
    # 实际展开后相当于：mysql -h127.0.0.1 -P3306 -uroot -p123456 app_db < your_file.sql
    # 意思是：连接到 app_db 数据库，并执行该 SQL 文件。
    mysql_cmd "$MYSQL_DB"< "$SQL_FILE"
}
### ======================


### ==== 4.进入MySQL 交互模式 ====
# 直接打开 MySQL 的交互式命令行（就像你在终端手动输入 mysql -u root -p 那样）。
mysql_shell() {
    echo "[INFO] 进入 MySQL Shell..."
    # mysql_cmd "$MYSQL_DB" 会展开为带数据库名的连接命令，所以进去后默认就在 app_db 里。
    mysql_cmd "$MYSQL_DB"
}
### ======================


### ==== 命令分发 ====
# case 是 Bash 的“多路分支”语句，类似编程语言中的 switch
# $1 是脚本的第一个参数（不是函数参数！）
# ;; 表示这个分支结束。
# 如果参数是 run，就检查 MySQL，然后调用 run_sql，并把第二个参数（$2）传给它。
#   ./db.sh run data/fix.sql
#   $1 = run，$2 = data/fix.sql
case "$1" in
    init_db)
        # 依次调用 check_mysql 和 init_db 函数。
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