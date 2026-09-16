package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func invokeExactArgs(t *testing.T, cmd *cobra.Command, n int, args []string) (stderr string, stdout string, err error) {
	t.Helper()
	var errBuf, outBuf strings.Builder
	cmd.SetErr(&errBuf)
	cmd.SetOut(&outBuf)
	// Call the validator directly. cmd.Execute() walks to the command-tree
	// root, so a child attached to the real CLI would run rootCmd.
	err = exactArgs(n)(cmd, args)
	return errBuf.String(), outBuf.String(), err
}

func TestExactArgsMissingIDPointsAtParentList(t *testing.T) {
	parent := &cobra.Command{Use: "skill"}
	parent.AddCommand(&cobra.Command{
		Use: "list",
		Run: func(*cobra.Command, []string) {},
	})
	get := &cobra.Command{
		Use:  "get",
		Args: exactArgs(1),
		Run:  func(*cobra.Command, []string) {},
	}
	parent.AddCommand(get)

	stderr, stdout, err := invokeExactArgs(t, get, 1, nil)
	if err != errSilent {
		t.Fatalf("err = %v, want errSilent", err)
	}
	if !strings.Contains(stderr, "Error: accepts 1 arg, received 0") {
		t.Fatalf("stderr missing arity error: %q", stderr)
	}
	if !strings.Contains(stderr, "See also: skill list") {
		t.Fatalf("stderr missing list next step: %q", stderr)
	}
	if !strings.Contains(strings.ToUpper(stdout), "USAGE") {
		t.Fatalf("stdout missing usage dump: %q", stdout)
	}
	seeIdx := strings.Index(stderr, "See also: skill list")
	if seeIdx < 0 {
		t.Fatal("expected See also before help")
	}
}

func TestExactArgsExtraArgDoesNotPointAtList(t *testing.T) {
	parent := &cobra.Command{Use: "skill"}
	parent.AddCommand(&cobra.Command{
		Use: "list",
		Run: func(*cobra.Command, []string) {},
	})
	get := &cobra.Command{
		Use:  "get",
		Args: exactArgs(1),
		Run:  func(*cobra.Command, []string) {},
	}
	parent.AddCommand(get)

	stderr, _, err := invokeExactArgs(t, get, 1, []string{"one", "two"})
	if err != errSilent {
		t.Fatalf("err = %v, want errSilent", err)
	}
	if !strings.Contains(stderr, "Error: accepts 1 arg, received 2") {
		t.Fatalf("stderr missing arity error: %q", stderr)
	}
	if strings.Contains(stderr, "See also:") {
		t.Fatalf("extra args must not hint list: %q", stderr)
	}
}

func TestExactArgsMissingArgWithoutParentListHasNoSeeAlso(t *testing.T) {
	parent := &cobra.Command{Use: "config"}
	set := &cobra.Command{
		Use:  "set",
		Args: exactArgs(1),
		Run:  func(*cobra.Command, []string) {},
	}
	parent.AddCommand(set)

	stderr, _, err := invokeExactArgs(t, set, 1, nil)
	if err != errSilent {
		t.Fatalf("err = %v, want errSilent", err)
	}
	if strings.Contains(stderr, "See also:") {
		t.Fatalf("parent without list must not hint list: %q", stderr)
	}
}

func TestExactArgsListCommandDoesNotPointAtItself(t *testing.T) {
	parent := &cobra.Command{Use: "files"}
	list := &cobra.Command{
		Use:  "list",
		Args: exactArgs(1),
		Run:  func(*cobra.Command, []string) {},
	}
	parent.AddCommand(list)

	stderr, _, err := invokeExactArgs(t, list, 1, nil)
	if err != errSilent {
		t.Fatalf("err = %v, want errSilent", err)
	}
	if strings.Contains(stderr, "See also:") {
		t.Fatalf("list itself must not hint list: %q", stderr)
	}
}

func TestSkillGetMissingIDPointsAtSkillList(t *testing.T) {
	origErr, origOut := skillGetCmd.ErrOrStderr(), skillGetCmd.OutOrStdout()
	t.Cleanup(func() {
		skillGetCmd.SetErr(origErr)
		skillGetCmd.SetOut(origOut)
		skillGetCmd.SetArgs(nil)
		rootCmd.SetArgs(nil)
	})

	stderr, stdout, err := invokeExactArgs(t, skillGetCmd, 1, nil)
	if err != errSilent {
		t.Fatalf("err = %v, want errSilent", err)
	}
	if !strings.Contains(stderr, "See also: skill list") && !strings.Contains(stderr, "See also: multica skill list") {
		t.Fatalf("stderr missing skill list next step: %q", stderr)
	}
	if strings.Contains(stderr+stdout, "See also: skill files list") {
		t.Fatalf("skill get must point at skill list, not files list: %q", stderr+stdout)
	}
}
