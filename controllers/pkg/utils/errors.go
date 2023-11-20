package utils

import (
	"fmt"

	"github.com/labring/sealos/controllers/pkg/code"
)

type ResourceShortageError struct {
	err error
}

func (e *ResourceShortageError) Error() string {
	return e.err.Error()
}

func NewResourceShortageError(err error) *ResourceShortageError {
	if err != nil {
		return &ResourceShortageError{err: fmt.Errorf(code.MessageFormat, code.ResourceShortageError, err.Error())}
	}
	return nil
}

func CheckResourceShortageError(err error) error {
	switch err.(type) {
	case *ResourceShortageError:
		return err
	default:
		return nil
	}
}
