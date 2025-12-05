package base

import (
	"errors"
	"fmt"
)

var (
	ErrKeyEmpty         = errors.New("empty key")
	ErrCacheMiss        = errors.New("cache miss")
	ErrInvalidConfig    = errors.New("invalid config")
	ErrSerialization    = errors.New("serialization error")
	ErrDeserialization  = errors.New("deserialization error")
	ErrTimeout          = errors.New("timeout")
	ErrConnectionFailed = errors.New("connection failed")
	ErrLockAcquisition  = errors.New("lock acquisition failed")
	ErrLockNotHeld      = errors.New("lock not held")
)

type Error struct {
	Op  string
	Err error
	Key string
}

func (e *Error) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("%s for key '%s': %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewError(op string, err error, key string) error {
	return &Error{Op: op, Err: err, Key: key}
}

func IsError(err error, target error) bool {
	return errors.Is(err, target)
}

func IsCacheMiss(err error) bool       { return IsError(err, ErrCacheMiss) }
func IsTimeout(err error) bool         { return IsError(err, ErrTimeout) }
func IsConnectionError(err error) bool { return IsError(err, ErrConnectionFailed) }
func IsLockError(err error) bool {
	return IsError(err, ErrLockAcquisition) || IsError(err, ErrLockNotHeld)
}
