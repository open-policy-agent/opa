package env

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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

func (cf cmdFlagsImpl) CheckEnvironmentVariables(command *cobra.Command) error {
	var errs []string
	v := viper.New()
	v.AutomaticEnv()
	if command.Name() == globalPrefix {
		v.SetEnvPrefix(command.Name())
	} else {
		v.SetEnvPrefix(fmt.Sprintf("%s_%s", globalPrefix, command.Name()))
	}
	command.Flags().VisitAll(func(f *pflag.Flag) {
		configName := f.Name
		configName = strings.ReplaceAll(configName, "-", "_")
		if !f.Changed && v.IsSet(configName) {
			val := v.Get(configName)
			err := command.Flags().Set(f.Name, fmt.Sprintf("%v", val))
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
	})

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", errorMessagePrefix, strings.Join(errs, "; "))
}
