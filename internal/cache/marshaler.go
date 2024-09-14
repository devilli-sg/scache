package cache

import (
	"reflect"

	jsoniter "github.com/json-iterator/go"
	"google.golang.org/protobuf/proto"
)

type marshaler[V any] interface {
	marshal(v V) ([]byte, error)
	unmarshal([]byte) (V, error)
}

// 非pb格式用json序列化
type jsonMarshal[V any] struct {
}

func (j *jsonMarshal[V]) marshal(v V) ([]byte, error) {
	return jsoniter.Marshal(v)
}
func (j *jsonMarshal[V]) unmarshal(data []byte) (V, error) {
	var v V
	err := jsoniter.Unmarshal(data, &v)
	return v, err
}

type strMarshal[V any] struct {
}

func (j *strMarshal[V]) marshal(v V) ([]byte, error) {
	switch t := any(v).(type) {
	case string:
		return []byte(t), nil
	case []byte:
		return t, nil
	}
	return nil, nil
}
func (j *strMarshal[V]) unmarshal(data []byte) (V, error) {
	var v V
	switch any(v).(type) {
	case string:
		return any(string(data)).(V), nil
	case []byte:
		return any(data).(V), nil
	}
	return v, nil
}

// pb格式用 pb 序列化，性能比json好一个数量级以上
type protoMarshal[V any] struct {
	base      reflect.Type
	isPointer bool
}

func (j *protoMarshal[V]) marshal(v V) ([]byte, error) {
	if j.isPointer {
		return proto.Marshal(any(v).(proto.Message))
	}
	return proto.Marshal(any(&v).(proto.Message))
}
func (j *protoMarshal[V]) unmarshal(data []byte) (V, error) {
	v := reflect.New(j.base)
	err := proto.Unmarshal(data, v.Interface().(proto.Message))
	if j.isPointer {
		return v.Interface().(V), err
	}
	return v.Elem().Interface().(V), err
}
func newProtoMarshaler[V any]() *protoMarshal[V] {
	m := &protoMarshal[V]{}
	var v V
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		m.isPointer = true
		m.base = t.Elem()
	} else {
		m.base = t
	}
	return m
}

func newMarshaler[V any]() marshaler[V] {
	var v V
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		if _, ok := any(v).(proto.Message); ok {
			return newProtoMarshaler[V]()
		}
	} else {
		if t.Kind() == reflect.String || t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return &strMarshal[V]{}
		}
		var vp *V
		t = reflect.TypeOf(vp)
		if t.Kind() == reflect.Pointer {
			if _, ok := any(vp).(proto.Message); ok {
				return newProtoMarshaler[V]()
			}
		}
	}
	return &jsonMarshal[V]{}
}
