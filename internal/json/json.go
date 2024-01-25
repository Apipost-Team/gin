// Copyright 2017 Bo-Yi Wu. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build !jsoniter && !go_json && !(sonic && avx && (linux || windows || darwin) && amd64)

package json

import (
	"fmt"
	"reflect"
	"strconv"
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
	var err error
	if len(str) > 16 {
		i, err = strconv.ParseInt(str, 10, 64)
		if err != nil {
			i = 0
		}
	} else {
		i, err = strconv.ParseInt(str, 16, 64)
		if err != nil {
			i = 0
		}
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

// 空数组64位数组
type EmptyArrayInt64Encoder struct {
	encoder jsoniter.ValEncoder
	decoder jsoniter.ValDecoder
}

func (encoder *EmptyArrayInt64Encoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	// If the pointer points to nil, write an empty object.
	if *(*uintptr)(ptr) == 0 {
		stream.WriteRaw("[]")
		return
	}

	//循环数组
	slice := (*[]int64)(ptr)
	strSlice := make([]string, len(*slice))
	for i, v := range *slice {
		if v == 0 {
			strSlice[i] = "0"
		} else {
			strSlice[i] = fmt.Sprintf("%016x", v)
		}
	}

	jsonData, err := jsonInstance.Marshal(strSlice)
	if err != nil {
		encoder.encoder.Encode(ptr, stream)
		return
	}
	stream.WriteRaw(string(jsonData))
}

func (codec *EmptyArrayInt64Encoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	//str := iter.ReadString()
	valueList := []int64{}
	for iter.ReadArray() {
		val := iter.Read()
		if iter.Error != nil {
			break
		}

		if str, ok := val.(string); ok {
			if len(str) > 16 {
				intVal, err := strconv.ParseInt(str, 10, 64)
				if err != nil {
					continue
				}
				valueList = append(valueList, intVal)
			} else {
				intVal, err := strconv.ParseInt(str, 16, 64)
				if err != nil {
					continue
				}
				valueList = append(valueList, intVal)
			}
		}
	}

	// 将ptr解析为*[]int64类型的指针
	ptrToSlice := (*[]int64)(ptr)
	// 使用reflect包将ptr的内容替换为slice
	reflect.ValueOf(ptrToSlice).Elem().Set(reflect.ValueOf(valueList))
}

func (encoder *EmptyArrayInt64Encoder) IsEmpty(ptr unsafe.Pointer) bool {
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
			if binding.Field.Type().Type1().Elem().String() == "int64" {
				//强制转64数组
				int64SliceEncode := &EmptyArrayInt64Encoder{binding.Encoder, binding.Decoder}
				binding.Encoder = int64SliceEncode
				binding.Decoder = int64SliceEncode
			} else if strings.Contains(binding.Field.Tag().Get("json"), "emptyarray") {
				binding.Encoder = &EmptyArrayEncoder{binding.Encoder}
			}
		}
	}
}

var jsonInstance jsoniter.API = jsoniter.Config{}.Froze()

func init() {
	jsonInstance.RegisterExtension(&ApipostExtension{})
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
