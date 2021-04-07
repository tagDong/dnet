package dnet

import (
	"fmt"
	"github.com/yddeng/dutil/buffer"
	"io"
	"reflect"
)

type (
	//编解码器
	Codec interface {
		Encode(o interface{}) ([]byte, error)
		Decode(reader io.Reader) (interface{}, error)
	}
)

// default编解码器
// 消息 -- 格式: 消息头(消息len), 消息体

const (
	lenSize  = 2       // 消息长度（消息体的长度）
	headSize = lenSize // 消息头长度
	buffSize = 65535   // 缓存容量(与lenSize有关，2字节最大65535）
)

type defCodec struct {
	readBuf  *buffer.Buffer
	dataLen  uint16
	readHead bool
}

func newCodec() *defCodec {
	return &defCodec{
		readBuf: &buffer.Buffer{},
	}
}

//解码
func (decoder *defCodec) Decode(reader io.Reader) (interface{}, error) {
	for {
		msg, err := decoder.unPack()

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

func (decoder *defCodec) unPack() ([]byte, error) {

	if !decoder.readHead {
		if decoder.readBuf.Len() < headSize {
			return nil, nil
		}

		decoder.dataLen, _ = decoder.readBuf.ReadUint16BE()
		decoder.readHead = true
	}

	if decoder.readBuf.Len() < int(decoder.dataLen) {
		return nil, nil
	}

	data, _ := decoder.readBuf.ReadBytes(int(decoder.dataLen))

	decoder.readHead = false
	return data, nil
}

//编码
func (encoder *defCodec) Encode(o interface{}) ([]byte, error) {

	data, ok := o.([]byte)
	if !ok {
		return nil, fmt.Errorf("dnet:Encode interface{} is %s, need type []byte", reflect.TypeOf(o))
	}

	dataLen := len(data)
	if dataLen > buffSize-headSize {
		return nil, fmt.Errorf("dnet:Encode dataLen is too large,len: %d", dataLen)
	}

	msgLen := dataLen + headSize
	buff := buffer.NewBufferWithCap(msgLen)

	//写入data长度
	buff.WriteUint16BE(uint16(dataLen))
	//data数据
	buff.WriteBytes(data)

	return buff.Bytes(), nil
}
