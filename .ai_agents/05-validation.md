# 05-validation.md

## 自检
检查语法、编译、未使用 import/变量、返回值、nil、接口实现、调用链、风格一致性。

## 格式化
- Go 代码至少 `gofmt`
- 若项目用 `goimports`，优先 `goimports`
- 优先只格式化本次修改文件

## 编译 / 测试
- 优先编译受影响包
- 有测试先跑受影响范围
- 改动较大再扩大范围
- 执行 `go test` / `go test ./...` 前优先选择受影响业务包
- 默认排除 `/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/TestNotes/**` 学习/示例目录，不为该目录失败做修复或适配
- 若执行 `go test ./...` 因 `TestNotes` 失败，结论中明确标注为排除范围；业务包通过即可作为本次业务验证结果
- 除非用户明确要求，不单独运行 `TestNotes` 下的测试、lint 或编译检查

## 临时产物
- 不把缓存、测试产物、调试日志、二进制留在工程目录
- 缓存目录优先用系统临时目录或工程外目录
- 编译时使用 `-o /tmp/` 或其他非工程目录输出编译产物
- 确保 `.gitignore` 包含所有可能的编译产物

## 结论
- 未执行的 build / test / lint 不得说成已通过
- 未验证必须说明原因
