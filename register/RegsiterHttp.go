package register

import (
	"../synserver"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

//var regsiterSyn sync.Mutex

/**
  注册信息  接收客户端 把自己的 服务发现名称  ip  端口 传过来
 */
func Register(w http.ResponseWriter, req *http.Request){
	regist := synserver.Regsiter{}
	result := synserver.RegisterResult{}
	err := req.ParseForm()
	if err != nil {
		log.Println(err)
	}
	//获取get post参数
	pServer := getFormValue("server", req)
	pHost := getFormValue("host", req)
	pPort := getFormValue("port", req)
	if pServer == "" || pHost == "" || pPort == "" {
		result.Code = "N"
	}
	if result.Code == "N" {
		result.Msg = "请求参数不完整"
		bytes, _ := json.Marshal(result)
		_, err := fmt.Fprintf(w, string(bytes))
		if err != nil {
			log.Println(err)
		}
		return
	}
	regist.Host = pHost
	regist.Server = pServer
	regist.Port = pPort
	//将注册信息异步推到队列 异步封装成数据
	//push(regist)
	/**
	   需要做到半数节点同意
	 */
	ok := synserver.LogSynData(regist)
	if ok == "ok"{
		result.Code = "Y"
	}else{
		result.Code = "N"
		result.Msg = "集群同步失败"
	}
	b, err := json.Marshal(result)
	if err != nil {
	}
	_, err2 := fmt.Fprintf(w, string(b))
	if err2 != nil {
		log.Println(err)
	}
}

func getFormValue(p string, req *http.Request) string{
	v, f := req.Form[p]
	if !f {
		log.Println("..接收参数异常:"+p)
		return ""
	}
	return v[0]
}