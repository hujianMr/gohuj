package synserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var FLLOWER = "fllower"
var CANDIDATE = "candidate"
var LEADER = "leader"

var Port string

//集群节点列表
var serverNodes []ServerNode


//配置本机地址 配置远程地址 用来做数据同步
var LocalUrl string
var remoterUrls []string

//leader角色的url地址
var leaderUrl string

//角色  Fllower  Leader  Candidate (基于raft协议来实现分布式一致性)
var role string
var roleLock sync.Mutex

func updateRole(r string){
	defer roleLock.Unlock()
	roleLock.Lock()
	role = r
}

//定时选举 间隔
var electionTime int

//全局leader心跳更新当前时间
var vTime time.Time

//全局版本号 用来选举使用  最大版本号得才有机会成为leader  但是不代表一定能成为leader 因为可能多个节点都是最大版本号
var currentTerm int

/**
   所有的节点承诺不接收小于他版本号（任期号）的选举请求
 */
//接收选举请求 http请求
func election(w http.ResponseWriter, req *http.Request){
	err := req.ParseForm()
	if err != nil {
		log.Println(err)
	}
	vTerm := getFormValue("term", req)
	vUrl := getFormValue("url", req)
	fmt.Println("参与选举url="+vUrl+"  任期号:"+ vTerm)
	pTerm, _ := strconv.Atoi(vTerm)
	electionResp := ElectionInfo{}
	electionResp.Version = currentTerm
	if pTerm < currentTerm || leaderUrl != "" {
		electionResp.Ok = false
	}else{
		electionResp.Ok = true
	}
	electionResp.Url = LocalUrl
	electionResp.Leader = role == LEADER
	bytes, _ := json.Marshal(electionResp)
	res := string(bytes)
	fmt.Println(time.Now().String()+"选举返回响应"+res)
	_, err2 := fmt.Fprintf(w,  res)
	if err2 != nil {
		log.Println(err)
	}
}

//心跳检测 http请求
func heartbeat(w http.ResponseWriter, req *http.Request){
	err := req.ParseForm()
	if err != nil {
		log.Println(err)
	}
	leaderUrl = getFormValue("leaderUrl", req)
	vTerm := getFormValue("term", req)
	pTerm, _ := strconv.Atoi(vTerm)
	if pTerm != currentTerm {
		currentTerm = pTerm
	}
	fmt.Println("接收leader心跳url:"+leaderUrl+" 任期号:"+ vTerm)
	vTime = time.Now()
	_, err2 := fmt.Fprintf(w,  "ok")
	if err2 != nil {
		log.Println(err)
	}
}

func getFormValue(p string, req *http.Request) string{
	v, f := req.Form[p]
	if !f {
		log.Println("..接收参数异常:"+p+"  ")
		return ""
	}
	return v[0]
}



//定时选举
func RegularElections()  {
	for{
		if electionTime < 100 {
			electionTime += 100
		}
		var dr = time.Duration(electionTime) * time.Millisecond
		time.Sleep(dr)
		//自身为leader不参与选举
		if role == LEADER {
			continue
		}
		//leader 等于空得情况  初始化 还没进行选举  状态改为 Candidate 发起选举
		if leaderUrl == "" {
			//更新为 candidate  发起选举
			updateRole(CANDIDATE)
			//调用选举
			runElection()
		}else{
			//获取时间差 秒数  关于秒数可以进行调整  这里暂时写死  需要大于leader的心跳时间
			timeDiff := time.Now().Sub(vTime).Seconds()
			//如果时间差相差15秒得时候  认为leader脑裂  发起广播请求
			if timeDiff > 15 {
				//如果该节点检测到leader心跳超时 置空leaderUrl
				leaderUrl = ""
				//更新未 candidate  发起选举
				updateRole(CANDIDATE)
				runElection()
			}
		}
	}
}

type ElectionInfo struct {
	Ok bool
	Version int
	Leader bool
	Url string
}

func asyncSendElection(serverNode ServerNode, electionSign *sync.WaitGroup, channel chan ElectionInfo){
	defer electionSign.Done()
	fullUrl := "http://"+serverNode.Url+"/election"
	params := url.Values{}
	Url, _:= url.Parse(fullUrl)
	params.Set("term", strconv.Itoa(currentTerm))
	params.Set("url", LocalUrl)
	//如果参数中有中文参数,这个方法会进行URLEncode
	Url.RawQuery = params.Encode()
	urlPath := Url.String()
	res, err := http.Get(urlPath)
	if res != nil && err == nil && res.StatusCode == 200 {
		serverNode.Active = false
		body,_ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		var electionInfo ElectionInfo
		_ = json.Unmarshal(body, &electionInfo)
		channel <- electionInfo
		_ = res.Body.Close()
	}
}

func runElection(){
	//serverNodes保存节点信息  可能有的节点失效了  不考虑 但是里面的地址是对的就行   leader里面的节点信息是最实时的
	//一旦该节点被选举成为了leader会自动异步发送心跳给其他节点
	// 发送选举 只需要告诉版本号  一旦有高版本号返回不接受 那么将进入下一个sleep之后下一次再发起选举
	serverLen := len(serverNodes)
	channel := make(chan ElectionInfo, serverLen)
	//每次发起选举自增一个 版本号  也就是任期号
	var electionSign sync.WaitGroup
	electionSign.Add(serverLen)
	//异步获取所有请求结果
	for i := 0; i < serverLen; i++{
		go asyncSendElection(serverNodes[i], &electionSign, channel)
	}
	electionSign.Wait()
	close(channel)
	//
	halfPOne := serverLen / 2 + 1
	currentTerm++
	for res := range channel {
		//表示同意成为当前leader
		if res.Ok {
			serverLen --
		}
		//只要有返回得版本号 大于当前帮版本号 那么直接退出竞选
		//有两种情况  大的版本号得 是会去参与选举得  所以他也会得到相同得票数  成为leader 这里等待心跳就行
		//还有一种情况 返回大得版本号得或者本身是leader 等待心跳就行
		if res.Version > currentTerm || res.Leader {
			fmt.Println("检测到有leader返回响应,"+res.Url)
			updateRole(FLLOWER)
			return
		}
	}
	//如果超过半数同意
	if halfPOne > serverLen {
		fmt.Println("超过半数投票成为leader,"+strconv.Itoa(serverLen))
		updateRole(LEADER)
		time.Sleep(2000)
		//synSignal.Done()
	}else{
		fmt.Println("少于半数投票成为fllower,"+strconv.Itoa(serverLen))
		updateRole(FLLOWER)
	}

}

//发送心跳检测 http请求
func sendHeartbeat(){
	for{
		//5秒发送心跳
		time.Sleep(5 * time.Second)
		//只有leader才能发送心跳给各个节点
		if !(role == LEADER) {
			continue
		}
		for i:=0;i<len(serverNodes);i++{
			serverNode := serverNodes[i]
			fullUrl := "http://"+serverNode.Url+"/heartbeat"
			params := url.Values{}
			Url, _:= url.Parse(fullUrl)
			params.Set("leaderUrl", LocalUrl)
			params.Set("term", strconv.Itoa(currentTerm))
			//如果参数中有中文参数,这个方法会进行URLEncode
			Url.RawQuery = params.Encode()
			urlPath := Url.String()
			res,_ := http.Get(urlPath)
			if res != nil {
				if res.StatusCode != 200 {
					serverNode.Active = false
				}else{
					body,_ := ioutil.ReadAll(res.Body)
					ok := string(body)
					fmt.Println("心跳url"+serverNode.Url+" 心跳返回:"+ok)
					if ok == "ok" {
						serverNode.Active = true
					}
				}
				serverNodes[i] = serverNode
				_ = res.Body.Close()
			}
		}
	}
}
