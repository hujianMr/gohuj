package synserver


var RegisterMap = make(map[string][]Regsiter)

/**
	注册中心结构
 */
type Regsiter struct {
	Host string  //服务地址
	Server string  //服务名称
	Port string       //服务端端口
}

/**
   注册结果返回对象  属性名首字母必须大写 否则无法转成json字符串
 */
type RegisterResult struct {
	Code string  //状态码  Y成功  N失败
	Msg string   //失败原因
}
