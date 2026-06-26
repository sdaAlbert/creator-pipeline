package app

import "errors"

var (
	ErrScriptNotReady = errors.New("script is only available after task succeeded")
	ErrTaskNotFound   = errors.New("task not found")
	ErrBadRewrite     = errors.New("bad rewrite request")
)
