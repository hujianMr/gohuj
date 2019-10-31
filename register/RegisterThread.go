package register

import (
	"../synserver"
	"../util"
	"strings"
	"sync"
)

var registerQueue = util.NewQueue()
var registerLock sync.Mutex
var threadLock sync.Mutex
//实现线程poll阻塞队列
var signal sync.WaitGroup

/**
   放入队列  保证线程安全  释放信号
 */
func push(r synserver.Regsiter){
	registerLock.Lock()
	registerQueue.Offer(r)
	registerLock.Unlock()
	signal.Done()
}

/**
  取出队列  保证线程安全 实现单线程阻塞等待
*/
func poll() synserver.Regsiter {
	for registerQueue.IsEmpty(){
		signal.Add(1)
		signal.Wait()
	}
	defer registerLock.Unlock()
	registerLock.Lock()
	element := registerQueue.Poll()
	value, _ := element.(synserver.Regsiter)
	return value
}

//不要求所有节点都同步 异步同步
func run(){
	for{
		reg := poll()
		fullUrl := reg.Host + ":" + reg.Port
		//地址指向常量map
		regMap := synserver.RegisterMap
		//根据服务名称取出地址列表集合
		regs := regMap[reg.Server]
		if len(regs) == 0{
			regs = append(regs, reg)
		}else{
			//根据服务名称对应的地址列表  做幂等
			var power bool
			for i:=0;i<len(regs);i++{
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
		data := synserver.SyncData{}
		data.ServerName = reg.Server
		data.Datas = regs
		synserver.Push(data)
	}
}
