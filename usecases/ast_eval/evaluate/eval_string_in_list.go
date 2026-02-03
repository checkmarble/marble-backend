package evaluate

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type StringInList struct {
	Function ast.Function
}

func NewStringInList(f ast.Function) StringInList {
	return StringInList{
		Function: f,
	}
}

func (f StringInList) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "Error in Evaluate function StringInList"))
	}
	if leftAny == nil || rightAny == nil {
		return nil, nil
	}

	anyList, errList := adaptArgumentToListOfThings[any](rightAny)
	if errList != nil {
		return MakeEvaluateError(errors.Wrap(errList, "right argument is not a list"))
	}
	if len(anyList) == 0 {
		return false, nil
	}

	switch anyList[0].(type) {
	case string:
		left, errLeft := adaptArgumentToString(leftAny)
		right, errRight := adaptArgumentToListOfStrings(rightAny)

		errs := MakeAdaptedArgsErrors([]error{errLeft, errRight})
		if len(errs) > 0 {
			return nil, errs
		}

		switch f.Function {
		case ast.FUNC_IS_IN_LIST:
			return stringInList(left, right), nil
		case ast.FUNC_IS_NOT_IN_LIST:
			return !stringInList(left, right), nil
		default:
			return MakeEvaluateError(errors.New(fmt.Sprintf(
				"StringInList does not support %s function", f.Function.DebugString())))
		}

	case netip.Prefix:
		left, errLeft := adaptArgumentToIp(leftAny)
		right, errRight := adaptArgumentToListOfCidrPrefixes(rightAny)

		errs := MakeAdaptedArgsErrors([]error{errLeft, errRight})
		if len(errs) > 0 {
			return nil, errs
		}

		switch f.Function {
		case ast.FUNC_IS_IN_LIST:
			return isIpInCidr(left, right), nil
		case ast.FUNC_IS_NOT_IN_LIST:
			return !isIpInCidr(left, right), nil
		default:
			return MakeEvaluateError(errors.New(fmt.Sprintf(
				"StringInList does not support %s function", f.Function.DebugString())))
		}
	}

	return nil, nil
}

func stringInList(str string, list []string) bool {
	return slices.Contains(list, str)
}

func isIpInCidr(ip netip.Addr, cidrs []netip.Prefix) bool {
	for _, cidr := range cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
