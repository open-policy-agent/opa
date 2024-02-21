package env

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func mockRootCmd(writer io.Writer) *cobra.Command {
	var rootArgs struct {
		IntFlag  int
		StrFlag  string
		BoolFlag bool
	}
	cmd := cobra.Command{
		Use:   "opa [opts]",
		Short: "test root command",
		Long:  `test root command`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return CmdFlags.CheckEnvironmentVariables(cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(writer, "%v; %v; %v", rootArgs.IntFlag, rootArgs.StrFlag, rootArgs.BoolFlag)
		},
	}
	cmd.Flags().IntVarP(&rootArgs.IntFlag, "int", "i", 0, "set int")
	cmd.Flags().StringVarP(&rootArgs.StrFlag, "some-string", "s", "", "set string")
	cmd.Flags().BoolVarP(&rootArgs.BoolFlag, "bool", "b", false, "set bool")
	return &cmd
}

func mockChildCmd(writer io.Writer) *cobra.Command {
	var rootArgs struct {
		IntFlag  int
		StrFlag  string
		BoolFlag bool
	}
	cmd := cobra.Command{
		Use:   "child [opts]",
		Short: "test child command",
		Long:  `test child command`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return CmdFlags.CheckEnvironmentVariables(cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(writer, "%v; %v; %v", rootArgs.IntFlag, rootArgs.StrFlag, rootArgs.BoolFlag)
		},
	}
	cmd.Flags().IntVarP(&rootArgs.IntFlag, "second-int", "i", 100, "set int")
	cmd.Flags().StringVarP(&rootArgs.StrFlag, "second-string", "s", "child-string", "set string")
	cmd.Flags().BoolVarP(&rootArgs.BoolFlag, "second-bool", "b", true, "set bool")
	return &cmd
}

func TestCmdFlagsImpl_CheckEnvironmentVariables_NoEnvVarsSingleCommand(t *testing.T) {
	rootWriter := bytes.NewBuffer([]byte{})
	root := mockRootCmd(rootWriter)
	if err := root.PreRunE(root, []string{}); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	root.Run(root, []string{})
	out := rootWriter.String()
	expectation := "0; ; false"
	if out != expectation {
		t.Fatalf("expected default flag values %q, got %q", expectation, out)
	}
}

func TestCmdFlagsImpl_CheckEnvironmentVariables_OneEnvVarsSingleCommand(t *testing.T) {
	rootWriter := bytes.NewBuffer([]byte{})
	root := mockRootCmd(rootWriter)
	t.Setenv("OPA_INT", "3")
	if err := root.PreRunE(root, []string{}); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	root.Run(root, []string{})
	out := rootWriter.String()
	expectation := "3; ; false"
	if out != expectation {
		t.Fatalf("expected flag values %q, got %q", expectation, out)
	}
}

func TestCmdFlagsImpl_CheckEnvironmentVariables_AllEnvVarsSingleCommand(t *testing.T) {
	rootWriter := bytes.NewBuffer([]byte{})
	root := mockRootCmd(rootWriter)
	t.Setenv("OPA_INT", "40")
	t.Setenv("OPA_SOME_STRING", "test")
	t.Setenv("OPA_BOOL", "true")
	if err := root.PreRunE(root, []string{}); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	root.Run(root, []string{})
	out := rootWriter.String()
	expectation := "40; test; true"
	if out != expectation {
		t.Fatalf("expected flag values %q, got %q", expectation, out)
	}
}

func TestCmdFlagsImpl_CheckEnvironmentVariables_ChildCommandAllEnvVars(t *testing.T) {
	root := mockRootCmd(&bytes.Buffer{})
	childWriter := bytes.NewBuffer([]byte{})
	child := mockChildCmd(childWriter)
	root.AddCommand(child)
	t.Setenv("OPA_CHILD_SECOND_INT", "7")
	t.Setenv("OPA_CHILD_SECOND_STRING", "testing child")
	t.Setenv("OPA_CHILD_SECOND_BOOL", "false")
	if err := child.PreRunE(child, []string{}); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	child.Run(child, []string{})
	childOut := childWriter.String()
	childExpectation := "7; testing child; false"
	if childOut != childExpectation {
		t.Fatalf("expected child flag values %q, got %q", childExpectation, childOut)
	}
}

func TestCmdFlagsImpl_CheckEnvironmentVariables_ChildCommandReturnsSingleErr(t *testing.T) {
	root := mockRootCmd(&bytes.Buffer{})
	child := mockChildCmd(&bytes.Buffer{})
	root.AddCommand(child)
	t.Setenv("OPA_CHILD_SECOND_BOOL", "7")
	err := child.PreRunE(child, []string{})
	if err == nil {
		t.Fatalf("expected error, found none")
	}
	expectedString := "invalid argument \"7\""
	if !strings.Contains(err.Error(), expectedString) {
		t.Fatalf("expected error to include %q, instead got %q", expectedString, err.Error())
	}
}

func TestCmdFlagsImpl_CheckEnvironmentVariables_ChildCommandReturnsMultipleErr(t *testing.T) {
	root := mockRootCmd(&bytes.Buffer{})
	child := mockChildCmd(&bytes.Buffer{})
	root.AddCommand(child)
	t.Setenv("OPA_CHILD_SECOND_INT", "true")
	t.Setenv("OPA_CHILD_SECOND_BOOL", "7")
	err := child.PreRunE(child, []string{"child"})
	expectedString := "invalid argument"
	if err == nil {
		t.Fatalf("expected error, found none")
	}
	if !strings.Contains(err.Error(), expectedString) {
		t.Fatalf("expected error to include %q, instead got %q", expectedString, err.Error())
	}
	if !strings.Contains(err.Error(), "7") {
		t.Fatalf("expected error for invalid int 7 as argument for boolean flag")
	}
	if !strings.Contains(err.Error(), "true") {
		t.Fatalf("expected error for invalid int 7 as argument for int flag")
	}
}

func TestCmdFlagsImpl_CheckEnvironmentVariables_ConfirmCommandFlagPrecedence(t *testing.T) {
	rootWriter := bytes.NewBuffer([]byte{})
	root := mockRootCmd(rootWriter)
	t.Setenv("OPA_INT", "3")
	t.Setenv("OPA_BOOL", "true")
	root.SetArgs([]string{"-i", "42"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	out := rootWriter.String()
	expectation := "42; ; true"
	if out != expectation {
		t.Fatalf("expected flag values %q, got %q", expectation, out)
	}
}
