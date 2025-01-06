package evaluate

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
)

type StringTemplate struct{}

func (f StringTemplate) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	if err := verifyNumberOfArguments(arguments.Args, 1); err != nil {
		return MakeEvaluateError(err)
	}

	if arguments.Args[0] == nil || arguments.Args[0] == "" {
		return nil, MakeAdaptedArgsErrors([]error{ast.ErrArgumentRequired})
	}

	template, templateErr := adaptArgumentToString(arguments.Args[0])
	if templateErr != nil {
		return MakeEvaluateError(templateErr)
	}

	var execErrors []error
	replacedTemplate := template
	variableRegexp := regexp.MustCompile(`(?mi)%([a-z0-9_]+)%`)
	for _, match := range variableRegexp.FindAllStringSubmatch(template, -1) {
		variableValue, argErr := adapatVariableValue(arguments.NamedArgs, match[1])
		if argErr != nil {
			if !ast.IsError(argErr, ast.ErrArgumentRequired) {
				execErrors = append(execErrors, argErr)
				continue
			}
			variableValue = "{}"
		}
		replacedTemplate = strings.Replace(replacedTemplate,
			fmt.Sprintf("%%%s%%", match[1]), variableValue, -1)
	}

	errs := MakeAdaptedArgsErrors(execErrors)
	if len(errs) > 0 {
		return nil, errs
	}

	return replacedTemplate, nil
}

func adapatVariableValue(namedArgs map[string]any, name string) (string, error) {
	if value, err := AdaptNamedArgument(namedArgs, name, adaptArgumentToString); err == nil {
		return value, nil
	}

	if value, err := AdaptNamedArgument(namedArgs, name, promoteArgumentToFloat64); err == nil {
		return strconv.FormatFloat(value, 'f', 2, 64), nil
	}

	if value, err := AdaptNamedArgument(namedArgs, name, promoteArgumentToInt64); err == nil {
		return strconv.FormatInt(value, 10), nil
	}

	return "", errors.Wrap(ast.ErrArgumentInvalidType,
		"all variables to String Template Evaluate must be string, int or float")
}
