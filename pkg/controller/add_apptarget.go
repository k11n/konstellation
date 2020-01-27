package controller

import (
	"github.com/davidzhao/konstellation/pkg/controller/apptarget"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, apptarget.Add)
}
