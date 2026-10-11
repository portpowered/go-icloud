package command

import (
	"context"
	"time"

	"github.com/urfave/cli/v3"
)

// commandFlags registers shared flag declarations with urfave; parsing, help,
// inherited flags and subcommand dispatch are owned by the framework.
type commandFlags struct{ flags []cli.Flag }

func (f *commandFlags) StringVar(destination *string, name, value, usage string) {
	var flag cli.StringFlag

	flag.Name, flag.Value, flag.Usage, flag.Destination = name, value, usage, destination
	f.flags = append(f.flags, &flag)
}
func (f *commandFlags) BoolVar(destination *bool, name string, value bool, usage string) {
	var flag cli.BoolFlag

	flag.Name, flag.Value, flag.Usage, flag.Destination = name, value, usage, destination
	f.flags = append(f.flags, &flag)
}
func (f *commandFlags) DurationVar(destination *time.Duration, name string, value time.Duration, usage string) {
	var flag cli.DurationFlag

	flag.Name, flag.Value, flag.Usage, flag.Destination = name, value, usage, destination
	f.flags = append(f.flags, &flag)
}
func (f *commandFlags) Func(name, usage string, action func(string) error) {
	var flag cli.StringSliceFlag

	flag.Name, flag.Usage = name, usage
	flag.Action = func(_ context.Context, _ *cli.Command, values []string) error {
		for _, value := range values {
			err := action(value)
			if err != nil {
				return err
			}
		}

		return nil
	}
	f.flags = append(f.flags, &flag)
}
