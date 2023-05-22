package models

import (
	"fmt"
)

// UnAuthorizedError is rendered with the http status code 400
type BadParameterError struct {
	Message string
	From    error
}

func (err *BadParameterError) Error() string {
	return fmt.Sprintf("Bad Parameter: %s", err.Message)
}

func (err *BadParameterError) Unwrap() error {
	return err.From
}

// UnAuthorizedError is rendered with the http status code 401
type UnAuthorizedError struct {
	Message string
	From    error
}

func (err *UnAuthorizedError) Error() string {
	return fmt.Sprintf("UnAuthorized: %s", err.Message)
}

func (err *UnAuthorizedError) Unwrap() error {
	return err.From
}

// UnAuthorizedError is rendered with the http status code 404
type NotFoundError struct {
	Message string
	From    error
}

func (err *NotFoundError) Error() string {
	return fmt.Sprintf("NotFound: %s", err.Message)
}

func (err *NotFoundError) Unwrap() error {
	return err.From
}
