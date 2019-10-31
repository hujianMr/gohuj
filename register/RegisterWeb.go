package register

import "net/http"

func RegisterWeb(){
	//注册路径
	http.HandleFunc("/register", Register)
	//异步开启线程
	go run()
}