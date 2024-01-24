// Copyright 2017 Bo-Yi Wu. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build !jsoniter && !go_json && !(sonic && avx && (linux || windows || darwin) && amd64)

package json

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
)

// HexStringEncoder 自定义编码器将 int64 类型编码为十六进制字符串或者把16进制转为int64
type HexStringEncoder struct{}

// Encode 实现 jsoniter.ValEncoder 接口
func (e *HexStringEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	if ptr == nil {
		stream.WriteNil()
		return
	}
	// Convert int64 value to a 16-byte hexadecimal string
	value := *(*int64)(ptr)
	if value == 0 {
		stream.WriteString("0") //0值特殊处理
	} else {
		stream.WriteString(fmt.Sprintf("%016x", value))
	}
}

func (e *HexStringEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return ptr == nil || *(*int64)(ptr) == 0
}

func (codec *HexStringEncoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	str := iter.ReadString()
	var i int64
	if len(str) > 16 {
		fmt.Sscanf(str, "%d", &i)
	} else {
		fmt.Sscanf(str, "%x", &i)
	}

	*((*int64)(ptr)) = i
}

// EmptyObjectEncoder 实现一个编码器，当字段值为nil时，写入空对象{}
type EmptyObjectEncoder struct {
	encoder jsoniter.ValEncoder
}

func (encoder *EmptyObjectEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	// If the pointer points to nil, write an empty object.
	if *(*uintptr)(ptr) == 0 {
		stream.WriteRaw("{}")
		return
	}
	// Fallback to default encoding.
	encoder.encoder.Encode(ptr, stream)
}

func (encoder *EmptyObjectEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.encoder.IsEmpty(ptr)
}

type EmptyArrayEncoder struct {
	encoder jsoniter.ValEncoder
}

func (encoder *EmptyArrayEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	// If the pointer points to nil, write an empty object.
	if *(*uintptr)(ptr) == 0 {
		stream.WriteRaw("[]")
		return
	}
	// Fallback to default encoding.
	encoder.encoder.Encode(ptr, stream)
}

func (encoder *EmptyArrayEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return encoder.encoder.IsEmpty(ptr)
}

// HexStringExtension 检查 struct 字段tags，为相应的 int64 字段应用 HexStringEncoder
type ApipostExtension struct {
	jsoniter.DummyExtension
}

// UpdateStructDescriptor 修改 struct 字段的编码/解码器
func (extension *ApipostExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		// 检查字段类型和 tag
		log.Println(binding.Field.Type().Kind())
		if binding.Field.Type().Kind() == reflect.Int64 {
			//处理64位转换
			if strings.Contains(binding.Field.Tag().Get("json"), "hexstring") {
				binding.Encoder = &HexStringEncoder{}
				binding.Decoder = &HexStringEncoder{}
			}
		} else if binding.Field.Type().Kind() == reflect.Ptr || binding.Field.Type().Kind() == reflect.Interface {
			//处理空对象
			if strings.Contains(binding.Field.Tag().Get("json"), "emptyobject") {
				binding.Encoder = &EmptyObjectEncoder{binding.Encoder}
			}
		} else if binding.Field.Type().Kind() == reflect.Slice || binding.Field.Type().Kind() == reflect.Array {
			//处理空数组
			if strings.Contains(binding.Field.Tag().Get("json"), "emptyarray") {
				binding.Encoder = &EmptyArrayEncoder{binding.Encoder}
			}
		}
	}
}

var jsonInstance jsoniter.API = jsoniter.Config{}.Froze()

func init() {
	jsonInstance.RegisterExtension(&JsonExtension{})
}

var (
	// Marshal is exported by gin/json package.
	Marshal = jsonInstance.Marshal
	// Unmarshal is exported by gin/json package.
	Unmarshal = jsonInstance.Unmarshal
	// MarshalIndent is exported by gin/json package.
	MarshalIndent = jsonInstance.MarshalIndent
	// NewDecoder is exported by gin/json package.
	NewDecoder = jsonInstance.NewDecoder
	// NewEncoder is exported by gin/json package.
	NewEncoder = jsonInstance.NewEncoder
)
