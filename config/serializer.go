package config

import (
	"encoding/json"
	"fmt"
)

// Options contains serialization options
type Options[T any] struct {
	Serializer Serializer[T]
}

// CacheOptions contains serialization options
type CacheOptions[T any] struct {
	Serializer JsonSerializer[T]
}

// Serializer provides serialization
type Serializer[T any] interface {
	Serialize(T) ([]byte, error)
	Deserialize([]byte) (T, error)
}

// JsonSerializer provides default JSON serialization
type JsonSerializer[T any] struct{}

// Serialize converts a value to JSON bytes
func (JsonSerializer[T]) Serialize(v T) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSerialization, err)
	}
	return b, nil
}

// Deserialize converts JSON bytes to a value
func (JsonSerializer[T]) Deserialize(data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("%w: %v", ErrDeserialization, err)
	}
	return v, nil
}

// BinarySerializer provides direct binary serialization for byte slices
type BinarySerializer struct{}

// Serialize returns the bytes as-is
func (BinarySerializer) Serialize(v []byte) ([]byte, error) {
	return v, nil
}

// Deserialize returns the bytes as-is
func (BinarySerializer) Deserialize(data []byte) ([]byte, error) {
	return data, nil
}

// StringSerializer provides direct string serialization
type StringSerializer struct{}

// Serialize converts string to bytes
func (StringSerializer) Serialize(v string) ([]byte, error) {
	return []byte(v), nil
}

// Deserialize converts bytes to string
func (StringSerializer) Deserialize(data []byte) (string, error) {
	return string(data), nil
}
