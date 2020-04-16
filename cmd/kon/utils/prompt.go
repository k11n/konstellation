package utils

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cast"
)

var (
	namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$`)
	BellSkipper = &bellSkipper{}
)

func ValidateInt(val string) error {
	_, err := cast.ToIntE(val)
	if err != nil {
		return fmt.Errorf("requires an int, received %s", val)
	}
	return nil
}

func ValidateIntWithLimits(min, max int) func(val string) error {
	return func(val string) error {
		num, err := cast.ToIntE(val)
		if err != nil {
			return fmt.Errorf("requires an int, received %s", val)
		}
		if min > -1 && num < min {
			return fmt.Errorf("requires a minimum of %d", min)
		}
		if max > -1 && num > max {
			return fmt.Errorf("no more than %d", max)
		}
		return nil
	}
}

func ValidateKubeName(val string) error {
	if namePattern.MatchString(val) {
		return nil
	} else {
		return fmt.Errorf("lowercase alphanumeric and - only")
	}
}

func SearchFuncFor(slice []string, requirePrefix bool) func(string, int) bool {
	return func(input string, idx int) bool {
		item := slice[idx]
		if requirePrefix {
			return strings.HasPrefix(item, input)
		} else {
			return strings.Contains(item, input)
		}
	}
}

func NewPromptSelect(label interface{}, items interface{}) promptui.Select {
	return promptui.Select{
		Label:  label,
		Items:  items,
		Stdout: BellSkipper,
	}
}

// bellSkipper implements an io.WriteCloser that skips the terminal bell
// character (ASCII code 7), and writes the rest to os.Stderr. It is used to
// replace readline.Stdout, that is the package used by promptui to display the
// prompts.
//
// This is a workaround for the bell issue documented in
// https://github.com/manifoldco/promptui/issues/49.
type bellSkipper struct{}

// Write implements an io.WriterCloser over os.Stderr, but it skips the terminal
// bell character.
func (bs *bellSkipper) Write(b []byte) (int, error) {
	const charBell = 7 // c.f. readline.CharBell
	if len(b) == 1 && b[0] == charBell {
		return 0, nil
	}
	return os.Stderr.Write(b)
}

// Close implements an io.WriterCloser over os.Stderr.
func (bs *bellSkipper) Close() error {
	return os.Stderr.Close()
}
