package file

import (
	"github.com/open-policy-agent/opa/v1/runtime"
)

const Name = "file_logger"

func init() {
	runtime.RegisterPlugin(Name, &Factory{})
}
