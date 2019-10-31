
##gohuj微服务注册中心  

###概述   
&nbsp;&nbsp;&nbsp;&nbsp;gohuj项目是使用go语言开发的一套微服务注册中心，基于raft协议来实现分布式一致性的leader选举

###实现的功能：  
1.基于raft协议leader选举  
2.服务注册  
3.服务发现  

###启动文件 
main包下面的Gohuj.go文件  

###基本设计   
&nbsp;&nbsp;&nbsp;&nbsp;服务可以单节点启动 也可以集群方式启动，通过配置文件application.in文件配置服务器列表   
&nbsp;&nbsp;&nbsp;&nbsp;register包下面主要实现服务注册功能，实现数据一致性的同步    
&nbsp;&nbsp;&nbsp;&nbsp;synserver包下面实现leader选举，基于raft协议来实现，实现了初始化选举，leader脑裂选举  
&nbsp;&nbsp;&nbsp;&nbsp;discover包下面主要提供了服务发现功能