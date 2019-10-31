package synserver

import (
	"github.com/Unknwon/goconfig/goconfig-master"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

type ServerNode struct {
	Url string
	Active bool
}

//设置服务节点列表
func setServerNodes(remoteUrls []string){
	for _, s := range remoteUrls {
		serverNode := ServerNode{s, true}
		serverNodes = append(serverNodes, serverNode)
	}
}

func InitConfig(fileName string){
	cfg, err := goconfig.LoadConfigFile(fileName)
	if err != nil{
		panic("找不到配置文件 application.ini")
	}
	address, err := cfg.GetValue("synServer", "gohuj.address")
	if err != nil {
		panic("gohuj.address参数配置为空")
	}
	remoterUrls = strings.Split(address, ",")
	setServerNodes(remoterUrls)
	//生成随机数  当没有leader的情况定时选举
	rand.Seed(time.Now().Unix())
	electionTime = rand.Intn(800)
	//服务刚启动的时候状态时fllower
	role = FLLOWER

	port, err := cfg.GetValue("synServer", "port")
	if err != nil {
		panic("port参数配置为空")
	}
	Port = port
	LocalUrl = localIP() + ":" + Port
}

func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func SynserverWeb(){
	http.HandleFunc("/synlogdata", synf)
	http.HandleFunc("/election", election)
	http.HandleFunc("/heartbeat", heartbeat)
	http.HandleFunc("/receiveLogData", receiveLogData)
	http.HandleFunc("/getLeaderUrl", getLeaderUrl)
	http.HandleFunc("/getServerSStatus", getServerSStatus)
	http.HandleFunc("/commitLogMap", commitLogMap)

	//异步定时  检测leader  发起选举
	go RegularElections()
	go sendHeartbeat()
	go Sendsynf()
}