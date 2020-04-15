package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cast"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$`)

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
