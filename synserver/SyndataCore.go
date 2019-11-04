package synserver

import (
	"../util"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var synQueue = util.NewQueue()
var synLock sync.Mutex

//实现线程poll阻塞队列
var signal sync.WaitGroup

var synSignal sync.WaitGroup

type SyncData struct {
	ServerName string
	Datas      []Regsiter
}

/**
  放入队列  保证线程安全  释放信号
*/
func Push(r SyncData) {
	synLock.Lock()
	synQueue.Offer(r)
	synLock.Unlock()
	signal.Done()
}

/**
  取出队列  保证线程安全 实现单线程阻塞等待
*/
func poll() SyncData {
	for synQueue.IsEmpty() {
		signal.Add(1)
		signal.Wait()
	}
	defer synLock.Unlock()
	synLock.Lock()
	element := synQueue.Poll()
	value, _ := element.(SyncData)
	return value
}

//接收同步leader节点数据  http请求
func synf(w http.ResponseWriter, req *http.Request) {
	result, _ := ioutil.ReadAll(req.Body)
	data := SyncData{}
	_ = json.Unmarshal(result, &data)
	RegisterMap[data.ServerName] = data.Datas
	_, _ = fmt.Fprint(w, "ok")
	_ = req.Body.Close()
}

/**
  异步开启同步数据
*/
/*func AsynBroadcast(serverNode ServerNode, data SyncData, broadcastSign *sync.WaitGroup, channel chan string) {
	defer broadcastSign.Done()
	bytesData, _ := json.Marshal(data)
	resp, _ := http.Post("http://"+serverNode.Url+"/synlogdata", "application/json", bytes.NewReader(bytesData))
	if resp != nil && resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("接收日志同步: " + string(body))
		channel <- string(body)
		_ = resp.Body.Close()
	} else {
		channel <- "NO"
	}
}*/

//发送同步leader节点数据  http请求  异步   要做到至少需要半数节点同意  不在这里实现  这里是客户端发送请求直接认为成功 后台异步同步给fllower
func Sendsynf() {
	for {
		data := poll()
		serverLen := len(serverNodes)
		channel := make(chan string, serverLen)
		var broadcastSign sync.WaitGroup
		broadcastSign.Add(serverLen)
		//异步获取所有请求结果
		for i := 0; i < serverLen; i++ {
			go func(serverNode ServerNode, data SyncData, broadcastSign *sync.WaitGroup, channel chan string) {
				defer broadcastSign.Done()
				bytesData, _ := json.Marshal(data)
				resp, _ := http.Post("http://"+serverNode.Url+"/synlogdata", "application/json", bytes.NewReader(bytesData))
				if resp != nil && resp.StatusCode == 200 {
					body, _ := ioutil.ReadAll(resp.Body)
					fmt.Println("接收日志同步: " + string(body))
					channel <- string(body)
					_ = resp.Body.Close()
				} else {
					channel <- "NO"
				}
			}(serverNodes[i], data, &broadcastSign, channel)
			//go AsynBroadcast(serverNodes[i], data, &broadcastSign, channel)
		}
		broadcastSign.Wait()
		close(channel)
		halfPOne := serverLen/2 + 1
		for res := range channel {
			if res == "ok" {
				serverLen--
			}
		}
		//超过半数同意才自增版本号
		if halfPOne > serverLen {
			currentTerm++
		}
	}
}

var logDataLock sync.Mutex

/**
  需要集群当中半数同意
*/
func LogSynData(regsiter Regsiter) string {
	logDataLock.Lock()
	serverLen := len(serverNodes)
	channel := make(chan string, serverLen)
	var broadcastSign sync.WaitGroup
	broadcastSign.Add(serverLen)
	currentTerm++
	fmt.Println("同步日志：" + regsiter.Server + "   " + regsiter.Host + "    " + regsiter.Port)
	//异步获取所有请求结果
	for i := 0; i < serverLen; i++ {
		go broadcast(serverNodes[i], regsiter, &broadcastSign, channel)
	}
	broadcastSign.Wait()
	close(channel)
	halfPOne := serverLen/2 + 1
	tempLen := len(serverNodes)
	for res := range channel {
		if res == "ok" {
			tempLen--
		}
	}
	fmt.Println("节点数：" + strconv.Itoa(serverLen) + " 未同步数：" + strconv.Itoa(serverLen-tempLen))
	ok := "NO"
	//超过半数同意才自增版本号
	if halfPOne > tempLen {
		ok = "ok"
		broadcastSign.Add(serverLen)
		for i := 0; i < serverLen; i++ {
			go commit(serverNodes[i], &broadcastSign)
		}
		broadcastSign.Wait()
	}
	logDataLock.Unlock()
	return ok
}

func commit(serverNode ServerNode, broadcastSign *sync.WaitGroup) {
	defer broadcastSign.Done()
	fullUrl := "http://" + serverNode.Url + "/commitLogMap"
	params := url.Values{}
	Url, _ := url.Parse(fullUrl)
	params.Set("term", strconv.Itoa(currentTerm))
	//如果参数中有中文参数,这个方法会进行URLEncode
	Url.RawQuery = params.Encode()
	urlPath := Url.String()
	res, err := http.Get(urlPath)
	if res != nil && err == nil && res.StatusCode == 200 {
	}
	_ = res.Body.Close()
}

type SynLog struct {
	Term string
	Reg  Regsiter
}

func broadcast(serverNode ServerNode, regsiter Regsiter, broadcastSign *sync.WaitGroup, channel chan string) {
	defer broadcastSign.Done()
	req := SynLog{strconv.Itoa(currentTerm), regsiter}
	bytesData, _ := json.Marshal(req)
	resp, _ := http.Post("http://"+serverNode.Url+"/receiveLogData",
		"application/json", bytes.NewReader(bytesData))
	if resp != nil && resp.StatusCode == 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("接收日志同步: " + string(body))
		channel <- string(body)
		_ = resp.Body.Close()
	} else {
		channel <- "NO"
	}
}

/**
  每个leader的新任期号会同步一次数据
*/
var logMap = make(map[string]Regsiter)

func commitLogMap(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Println(err)
	}
	term := getFormValue("term", req)
	reg := logMap[term]
	fullUrl := reg.Host + ":" + reg.Port
	//地址指向常量map
	regMap := RegisterMap
	//根据服务名称取出地址列表集合
	regs := regMap[reg.Server]
	if len(regs) == 0 {
		regs = append(regs, reg)
	} else {
		//根据服务名称对应的地址列表  做幂等
		var power bool
		for i := 0; i < len(regs); i++ {
			mfullUrl := regs[i].Host + ":" + regs[i].Port
			//如果有相同的url了
			if strings.Compare(fullUrl, mfullUrl) == 0 {
				power = true
				break
			}
		}
		//如果没有 追加
		if !power {
			regs = append(regs, reg)
		}
	}
	regMap[reg.Server] = regs
}

//接收同步leader节点数据  http请求
func receiveLogData(w http.ResponseWriter, req *http.Request) {
	result, _ := ioutil.ReadAll(req.Body)
	reg := SynLog{}
	_ = json.Unmarshal(result, &reg)
	fmt.Println("同步日志：" + reg.Reg.Server + "   " + reg.Reg.Host + "    " + reg.Reg.Port + "   任期号: " + reg.Term)
	logMap[reg.Term] = reg.Reg
	_, _ = fmt.Fprint(w, "ok")
	_ = req.Body.Close()
}
