package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cast"
)

var namePattern = regexp.MustCompile(`^[a-z0-9\-\.]+$`)

func ValidateInt(val string) error {
	_, err := cast.ToIntE(val)
	if err != nil {
		return fmt.Errorf("requires an int, received %s", val)
	}
	return nil
}

func ValidateName(val string) error {
	if namePattern.MatchString(val) {
		return nil
	} else {
		return fmt.Errorf("alphanumeric, -, and . only")
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
