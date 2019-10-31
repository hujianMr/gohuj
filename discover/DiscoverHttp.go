package discover

import (
	"../synserver"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func DiscoverHttp() {
	http.HandleFunc("/listByServer", listByServer)
	http.HandleFunc("/listServer", listServer)
}


func listByServer(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Println(err)
	}
	server := getFormValue("server", req)
	reg := synserver.RegisterMap[server]
	bytes,_ := json.Marshal(reg)
	_,_ = fmt.Fprintf(w, string(bytes))
}

func listServer(w http.ResponseWriter, req *http.Request) {
	bytes,_ := json.Marshal(synserver.RegisterMap)
	_,_ = fmt.Fprintf(w, string(bytes))
}

func getFormValue(p string, req *http.Request) string{
	v, f := req.Form[p]
	if !f {
		log.Println("..接收参数异常:"+p+"  ")
		return ""
	}
	return v[0]
}