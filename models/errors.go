package models

import (
	"errors"
)

// BadParameterError is rendered with the http status code 400
var BadParameterError = errors.New("Bad Parameter")

// UnAuthorizedError is rendered with the http status code 401
var UnAuthorizedError = errors.New("Authorized")

// ForbiddenError is rendered with the http status code 403
var ForbiddenError = errors.New("Forbidden")

// NotFoundError is rendered with the http status code 404
var NotFoundError = errors.New("Not found")
