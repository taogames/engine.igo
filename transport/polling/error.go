package polling

import "errors"

var ErrUpgrade error = errors.New("Engine.IO transport upgrading")
var ErrClose error = errors.New("Engine.IO transport closed")
