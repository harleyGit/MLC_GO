package middleware_pt

import (
	"fmt"
	"net/http"
)

type Middleware func(http.Handler) http.Handler

func MiddlewareDemoMain() {
	fmt.Println("\n========================================")
	fmt.Println("中间件洋葱模型演示")
	fmt.Println("========================================")
	fmt.Println("启动服务器 :8080")
	fmt.Println("测试命令: curl http://localhost:8080/hello")
	fmt.Println("测试panic: curl http://localhost:8080/panic")
	fmt.Println("========================================\n")

	businessHello := http.HandlerFunc(helloHandler)
	businessPanic := http.HandlerFunc(panicHandler)

	chain := func(h http.Handler) http.Handler {
		return ChainInterceptors(h,
			RequestTIDInterceptor,
			AccessLogInterceptor,
			RecoverInterceptor,
			JSONHeaderInterceptor,
		)
	}

	http.Handle("/hello", chain(businessHello))
	http.Handle("/panic", chain(businessPanic))
	http.ListenAndServe(":8080", nil)
}

func ChainInterceptors(base http.Handler, middlewares ...Middleware) http.Handler {
	wrapped := base
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

func JSONHeaderInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("========================================")
		fmt.Println("【1-进入】JSONHeaderInterceptor - 设置响应头")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
		fmt.Println("【1-退出】JSONHeaderInterceptor - 响应已完成")
		fmt.Println("========================================")
	})
}

func RecoverInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("【2-进入】RecoverInterceptor - 设置 panic 捕获")
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("【2-捕获】RecoverInterceptor - panic: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"code":500,"message":"服务器内部错误"}`))
			}
		}()
		next.ServeHTTP(w, r)
		fmt.Println("【2-退出】RecoverInterceptor - 正常完成")
	})
}

func AccessLogInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("【3-进入】AccessLogInterceptor - %s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		fmt.Println("【3-退出】AccessLogInterceptor - 请求完成")
	})
}

func RequestTIDInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := "TID-20260430-ABC123"
		fmt.Printf("【4-进入】RequestTIDInterceptor - TID: %s\n", tid)
		next.ServeHTTP(w, r)
		fmt.Printf("【4-退出】RequestTIDInterceptor - TID: %s\n", tid)
	})
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(">>> 【业务处理】开始执行 <<<")
	w.Write([]byte(`{"code":0,"message":"Hello, World!"}`))
	fmt.Println(">>> 【业务处理】执行完毕 <<<")
}

func panicHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(">>> 【业务处理】即将 panic <<<")
	panic("模拟数据库连接失败")
}