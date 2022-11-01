package event

import "errors"

type Error error

var (
	ErrReadTimeout Error = errors.New("read timeout")
)
