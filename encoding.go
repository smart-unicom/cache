// Package cache 提供json、protobuf和gob的编码和解码功能
package cache

import (
	"encoding"
	"errors"
	"reflect"
	"strings"
)

var (
	// ErrNotAPointer 参数必须是指针类型错误
	ErrNotAPointer = errors.New("参数必须是指针类型")
)

// Codec 定义gRPC用于编码和解码消息的接口
// 注意：此接口的实现必须是线程安全的；Codec的方法可以从并发的goroutine中调用
type Codec interface {
	// Marshal 返回v的线格式
	Marshal(v interface{}) ([]byte, error)
	// Unmarshal 将线格式解析为v
	Unmarshal(data []byte, v interface{}) error
	// Name 返回Codec实现的名称
	// 返回的字符串将用作传输中内容类型的一部分
	// 结果必须是静态的；结果在调用之间不能改变
	Name() string
}

var registeredCodecs = make(map[string]Codec)

// RegisterCodec 注册提供的Codec以供所有传输客户端和服务器使用
//
// Codec将通过其Name()方法的结果进行存储和查找，
// 该结果应与Codec处理的编码的内容子类型匹配
// 这是不区分大小写的，并以小写形式存储和查找
// 如果调用Name()的结果是空字符串，RegisterCodec将panic
//
// 注意：此函数只能在初始化时调用（即在init()函数中），并且不是线程安全的
// 如果多个压缩器以相同名称注册，最后注册的将生效
func RegisterCodec(codec Codec) {
	if codec == nil {
		panic("cannot register a nil Codec")
	}
	if codec.Name() == "" {
		panic("cannot register Codec with empty string result for Name()")
	}
	contentSubtype := strings.ToLower(codec.Name())
	registeredCodecs[contentSubtype] = codec
}

// GetCodec 通过内容子类型获取已注册的Codec
// 如果没有为该内容子类型注册Codec，则返回nil
//
// 内容子类型应为小写
func GetCodec(contentSubtype string) Codec {
	return registeredCodecs[contentSubtype]
}

// Encoding 编码接口定义
type Encoding interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

// Marshal 编码数据
func Marshal(e Encoding, v interface{}) (data []byte, err error) {
	if !isPointer(v) {
		return data, ErrNotAPointer
	}
	bm, ok := v.(encoding.BinaryMarshaler)
	if ok && e == nil {
		return bm.MarshalBinary()
	}

	data, err = e.Marshal(v)
	if err == nil {
		return data, err
	}
	if ok {
		data, err = bm.MarshalBinary()
	}

	return data, err
}

// Unmarshal 解码数据
func Unmarshal(e Encoding, data []byte, v interface{}) (err error) {
	if !isPointer(v) {
		return ErrNotAPointer
	}
	bm, ok := v.(encoding.BinaryUnmarshaler)
	if ok && e == nil {
		err = bm.UnmarshalBinary(data)
		return err
	}
	err = e.Unmarshal(data, v)
	if err == nil {
		return err
	}
	if ok {
		return bm.UnmarshalBinary(data)
	}
	return err
}

func isPointer(data interface{}) bool {
	switch reflect.ValueOf(data).Kind() {
	case reflect.Ptr, reflect.Interface:
		return true
	default:
		return false
	}
}
