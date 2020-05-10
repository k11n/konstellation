package controller

import (
	"github.com/k11n/konstellation/pkg/controller/apprelease"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, apprelease.Add)
}
