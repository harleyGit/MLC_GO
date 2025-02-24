/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-01-22 10:28:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-23 16:31:45
 * @FilePath: /MLC_GO/TestNotes/TestMySQL/TestMySQLV1/test_mysql_v1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"fmt"
	"log"

	// 当导入带有空白标识符前缀“_”的包时，将调用包的init()函数，以注册go-mysql驱动程序。
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

func main() {// 操作数据库前先启动数据库： sudo mysql.server start
	// 数据库版本
	testMysqlV1_version()

	// 新建数据表
	//TestMySQLV2_createTable()

	// 插入数据
	//testMySQLV2_insert()

	// 查询数据
	//testMySQLV2_query()

	// 修改数据
	// testMySQLV2_update()

	// 删除数据
	// testMySQLV2_delete()
}


// 删除数据
func testMySQLV2_delete() {
	// 创建数据对象
	db, err := sql.Open("mysql", "root:hh109@tcp(127.0.0.1:3306)/DB_TEST")
	db.Ping()        // 与数据库建立连接
	defer db.Close() // 延迟关闭数据库

	if err != nil {
		fmt.Println("数据库连接失败！")
		log.Fatal(err)
	}

	// 删除用户名字的SQL语句
	sql := "DELETE FROM user WHERE id = 1"
	_,err2 := db.Exec(sql)
	if err2 != nil {
		log.Fatal(err2)
	}
	fmt.Print("已成功删除数据表 user 中的数据！\n") //修改数据后，打印提示信息


	// 查询数据表 user 中的所有数据
	result,err3 := db.Query("SELECT * FROM user")
	if err3 != nil {
		log.Fatal(err3)
	}

	//遍历查询结果
	for result.Next() {
		var id int	// 主键id
		var name string	// 用户的名字

		err = result.Scan(&id, &name)
		if err != nil {
			panic(err)
		}
		fmt.Printf("id: %d, name: %s\n", id,name)
	}

}


// 修改数据
func testMySQLV2_update() {
	// 创建数据对象
	db, err := sql.Open("mysql", "root:hh109@tcp(127.0.0.1:3306)/DB_TEST")
	db.Ping()        // 与数据库建立连接
	defer db.Close() // 延迟关闭数据库

	if err != nil {
		fmt.Println("数据库连接失败！")
		log.Fatal(err)
	}

	// 修改用户名字的SQL语句
	sql := "update user set name = ? WHERE id = ?"
	_,err2 := db.Exec(sql, "张三🍔", 2)
	if err2 != nil {
		log.Fatal(err2)
	}
	fmt.Print("已成功修改数据表 user 中的数据！\n") //修改数据后，打印提示信息


	// 查询数据表 user 中的所有数据
	result,err3 := db.Query("SELECT * FROM user")
	if err3 != nil {
		log.Fatal(err3)
	}

	//遍历查询结果
	for result.Next() {
		var id int	// 主键id
		var name string	// 用户的名字

		err = result.Scan(&id, &name)
		if err != nil {
			panic(err)
		}
		fmt.Printf("id: %d, name: %s\n", id,name)
	}

}

// 查询数据
func testMySQLV2_query() {
	// 创建数据对象
	db, err := sql.Open("mysql", "root:hh109@tcp(127.0.0.1:3306)/DB_TEST")
	db.Ping()        // 与数据库建立连接
	defer db.Close() // 延迟关闭数据库

	if err != nil {
		fmt.Println("数据库连接失败！")
		log.Fatal(err)
	}

	// 插入一条数据
	_,err2 := db.Query("INSERT INTO user VALUES(2, '司马懿🍎')")
	if err2 != nil {
		log.Fatal(err2)
	}

	// 查询数据表 user 中的所有数据
	result,err3 := db.Query("SELECT * FROM user")
	if err3 != nil {
		log.Fatal(err3)
	}

	//遍历查询结果
	for result.Next() {
		var id int	// 主键id
		var name string	// 用户的名字

		err = result.Scan(&id, &name)
		if err != nil {
			panic(err)
		}
		fmt.Printf("id: %d, name: %s\n", id,name)
	}

}

// 插入数据
func testMySQLV2_insert() {
	// 创建数据对象
	db, err := sql.Open("mysql", "root:hh109@tcp(127.0.0.1:3306)/DB_TEST")
	db.Ping()        // 与数据库建立连接
	defer db.Close() // 延迟关闭数据库

	if err != nil {
		fmt.Println("数据库连接失败！")
		log.Fatal(err)
	}

	_,err2 := db.Query("INSERT INTO user VALUES(1, 'David')")
	if err2 != nil {
		log.Fatal(err2)
	}
	fmt.Print("已成功向数据表 user 插入数据！\n")
}

// 数据库版本
func testMysqlV1_version() {
	// 创建数据对象
	db, err := sql.Open("mysql", "root:hh109@tcp(127.0.0.1:3306)/DB_TEST")
	db.Ping()        // 与数据库建立连接
	defer db.Close() // 延迟关闭数据库

	if err != nil {
		fmt.Println("数据库连接失败！")
		log.Fatal(err)
	}

	var version string                                     // 声明 MySQL 数据库版本
	err2 := db.QueryRow("SELECT VERSION()").Scan(&version) // 单行查询

	if err2 != nil {
		log.Fatal(err2)
	}

	fmt.Println(version) // 打印 MySQL 数据库版本
}

//  新建数据表user
func TestMySQLV2_createTable() {
	// 创建数据对象
	db, err := sql.Open("mysql", "root:hh109@tcp(127.0.0.1:3306)/DB_TEST")
	db.Ping()        // 与数据库建立连接
	defer db.Close() // 延迟关闭数据库

	if err != nil {
		fmt.Println("数据库连接失败！")
		log.Fatal(err)
	}

	// 执行SQL语句
	_,err2 := db.Exec("CREATE TABLE user(id INT NOT NULL, name VARCHAR(20), PRIMARY KEY(ID));")
	if err2 != nil {
		log.Fatal(err2)
	}
	
	fmt.Print("已成功新建数据表 user! \n") // 打印 MySQL 数据库版本
}



