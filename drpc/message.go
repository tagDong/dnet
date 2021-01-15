package drpc

type Request struct {
	SeqNo  uint64
	Method string // 请求方法名
	Data   interface{}
}

type Response struct {
	SeqNo uint64
	Data  interface{}
}

type RPCChannel interface {
	SendRequest(req *Request) error    // 发送RPC请求
	SendResponse(resp *Response) error // 发送RPC回复
}
