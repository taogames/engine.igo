package transport

import "errors"

var ErrUpgrade error = errors.New("Engine.IO transport upgrading")
var ErrClosed error = errors.New("Engine.IO transport closed")
