package config

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrSerialization   = errors.New("serialization error")
	ErrDeserialization = errors.New("deserialization error")
)

type Serializer[T any] interface {
	Encode(T) ([]byte, error)
	Decode([]byte) (T, error)
}

type JsonSerializer[T any] struct{}

func (JsonSerializer[T]) Encode(v T) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSerialization, err)
	}
	return b, nil
}

func (JsonSerializer[T]) Decode(data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("%w: %v", ErrDeserialization, err)
	}
	return v, nil
}

type (
	BinarySerializer = IdentitySerializer[[]byte]
	StringSerializer = ConvertSerializer[string]
)

type IdentitySerializer[T any] struct{}

func (IdentitySerializer[T]) Encode(v T) ([]byte, error)    { return any(v).([]byte), nil }
func (IdentitySerializer[T]) Decode(data []byte) (T, error) { return any(data).(T), nil }

type ConvertSerializer[T ~string] struct{}

func (ConvertSerializer[T]) Encode(v T) ([]byte, error)    { return []byte(v), nil }
func (ConvertSerializer[T]) Decode(data []byte) (T, error) { return T(data), nil }
