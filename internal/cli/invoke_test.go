package cli

import (
	"strings"
	"testing"
)

func TestInvoke(t *testing.T) {
	var out strings.Builder
	Register(Command{Name: "invoke-test", NoConfig: true, Run: func(c *Ctx, args []string) error {
		out.WriteString(strings.Join(args, ","))
		return nil
	}})
	c := &Ctx{}
	if err := Invoke(c, "invoke-test", []string{"a", "b"}); err != nil || out.String() != "a,b" {
		t.Fatalf("%v %q", err, out.String())
	}
	err := Invoke(c, "no-such-command", nil)
	if err == nil || err.Error() != "no-such-command: not available" {
		t.Fatalf("%v", err)
	}
	if cmd, ok := Lookup("invoke-test"); !ok || cmd.Name != "invoke-test" {
		t.Fatal("lookup")
	}
	if _, ok := Lookup("zzz"); ok {
		t.Fatal("lookup miss")
	}
}
