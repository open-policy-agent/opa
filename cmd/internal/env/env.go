package env

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type cmdFlags interface {
	CheckEnvironmentVariables(command *cobra.Command) error
}

type cmdFlagsImpl struct{}

var (
	CmdFlags           cmdFlags = cmdFlagsImpl{}
	errorMessagePrefix          = "error mapping environment variables to command flags"
)

const globalPrefix = "opa"

func (cmdFlagsImpl) CheckEnvironmentVariables(command *cobra.Command) error {
	prefix := globalPrefix
	if command.Name() != globalPrefix {
		prefix = fmt.Sprintf("%s_%s", globalPrefix, command.Name())
	}
	prefix = strings.ToUpper(prefix) + "_"

	var errs []string
	command.Flags().VisitAll(func(f *pflag.Flag) {
		name := prefix + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		// Empty variables are treated as unset.
		if val, ok := os.LookupEnv(name); ok && val != "" && !f.Changed {
			if err := command.Flags().Set(f.Name, val); err != nil {
				errs = append(errs, err.Error())
			}
		}
	})

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", errorMessagePrefix, strings.Join(errs, "; "))
}
