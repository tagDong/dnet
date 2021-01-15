package codec

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/yddeng/dnet/drpc"
	"github.com/yddeng/dutil/buffer"
	"io"
	"reflect"
)

// rpc编解码器
// 消息 -- 格式: 消息头(消息Seq+消息flag+协议名len+协议内容len), 消息体(协议名+协议内容)
// 仅支持protobuf

const (
	seqSize   = 8                                        // 消息的索引 //uint64
	flagSize  = 1                                        // 消息flag //byte
	nameSize  = 1                                        // 协议名长度 //uint8
	bodySize  = 2                                        // 协议内容长度（消息体的编码ID，对应的反序列化结构）//uint16
	rheadSize = seqSize + flagSize + nameSize + bodySize // 消息头长度
	rbuffSize = 65535                                    // 缓存容量(与lenSize有关，2字节最大65535）
)

type RpcCodec struct {
	readBuf *buffer.Buffer
	seqNo   uint64
	flag    byte
	name    string
	nameLen uint8
	bodyLen uint16
}

func NewRpcCodec() *RpcCodec {
	return &RpcCodec{
		readBuf: buffer.NewBufferWithCap(rbuffSize),
	}
}

type flag byte

const (
	message flag = 0x80
	rpcReq  flag = 0x40
	rpcResp flag = 0x20
)

func (this flag) setType(tt flag) flag {
	switch tt {
	case message:
		return this | message
	case rpcReq:
		return this | rpcReq
	case rpcResp:
		return this | rpcResp
	default:
		panic("invalid type")
	}
}

func (this flag) getType() flag {
	if this&message > 0 {
		return message
	} else if this&rpcReq > 0 {
		return rpcReq
	} else if this&rpcResp > 0 {
		return rpcResp
	}
	return message
}

//解码
func (decoder *RpcCodec) Decode(reader io.Reader) (interface{}, error) {
	for {
		msg, err := decoder.unPack()

		//fmt.Println(msg, err)
		if msg != nil {
			return msg, nil

		} else if err == nil {
			_, err1 := decoder.readBuf.ReadFrom(reader)
			if err1 != nil {
				return nil, err1
			}
		} else {
			return nil, err
		}
	}
}

func (decoder *RpcCodec) unPack() (interface{}, error) {
	if decoder.bodyLen == 0 {
		if decoder.readBuf.Len() < rheadSize {
			return nil, nil
		}

		decoder.seqNo, _ = decoder.readBuf.ReadUint64BE()
		decoder.flag, _ = decoder.readBuf.ReadByte()
		decoder.nameLen, _ = decoder.readBuf.ReadUint8BE()
		decoder.bodyLen, _ = decoder.readBuf.ReadUint16BE()
	}

	var ret interface{}
	var err error
	tt := flag(decoder.flag).getType()
	switch tt {
	case rpcReq:
		name, _ := decoder.readBuf.ReadString(int(decoder.nameLen))
		body, _ := decoder.readBuf.ReadBytes(int(decoder.bodyLen))

		msg, err := Unmarshal(name, body)
		if err != nil {
			return nil, err
		}
		ret = &drpc.Request{
			SeqNo:  decoder.seqNo,
			Method: name,
			Data:   msg,
		}
	case rpcResp:
		resp := &drpc.Response{SeqNo: decoder.seqNo}

		name, _ := decoder.readBuf.ReadString(int(decoder.nameLen))
		body, _ := decoder.readBuf.ReadBytes(int(decoder.bodyLen))

		msg, err := Unmarshal(name, body)
		if err != nil {
			return nil, err
		}
		resp.Data = msg

		ret = resp
	case message:
	default:
		err = fmt.Errorf("unPack err: flag is %d", decoder.flag)
	}

	//将消息长度置为0，用于下一次验证
	decoder.bodyLen = 0
	return ret, err
}

//编码
func (encoder *RpcCodec) Encode(o interface{}) ([]byte, error) {
	var seqNo uint64
	var flag flag
	var name string
	var data []byte
	var nameLen, bodyLen int
	var err error

	switch o.(type) {
	case *drpc.Request:
		request := o.(*drpc.Request)
		seqNo = request.SeqNo
		flag = flag.setType(rpcReq)

		name, data, err = Marshal(request.Data)
		if err != nil {
			return nil, err
		}
	case *drpc.Response:
		response := o.(*drpc.Response)
		seqNo = response.SeqNo
		flag = flag.setType(rpcResp)

		name, data, err = Marshal(response.Data)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("encode error , o'type is %s", reflect.TypeOf(o).String())
	}

	nameLen = len(name)
	bodyLen = len(data)
	if bodyLen+nameLen > rbuffSize-rheadSize {
		return nil, fmt.Errorf("encode dataLen is too large,len: %d", bodyLen+nameLen)
	}

	msgLen := rheadSize + nameLen + bodyLen
	buff := buffer.NewBufferWithCap(msgLen)

	//写入seqNo
	buff.WriteUint64BE(seqNo)
	//flag
	buff.WriteByte(byte(flag))
	//namelen
	buff.WriteUint8BE(uint8(nameLen))
	//bodylen
	buff.WriteUint16BE(uint16(bodyLen))
	//name
	buff.WriteString(name)
	//body
	buff.WriteBytes(data)

	return buff.Bytes(), nil
}

func Marshal(data interface{}) (string, []byte, error) {
	ret, err := proto.Marshal(data.(proto.Message))
	if err != nil {
		return "", nil, err
	}
	name := proto.MessageName(data.(proto.Message))
	return name, ret, nil
}

func Unmarshal(name string, data []byte) (msg interface{}, err error) {
	tt := proto.MessageType(name)
	//反序列化的结构
	msg = reflect.New(tt.Elem()).Interface()
	err = proto.Unmarshal(data, msg.(proto.Message))
	if err != nil {
		return nil, err
	}
	return msg, nil
}
