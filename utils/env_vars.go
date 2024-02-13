package utils

import (
	"log"
	"os"
	"reflect"
	"strconv"
)

type ParserFunc func(v string) (interface{}, error)

var (
	defaultBuiltInParsers = map[reflect.Kind]ParserFunc{
		reflect.String: func(v string) (interface{}, error) {
			return v, nil
		},
		reflect.Bool: func(v string) (interface{}, error) {
			return strconv.ParseBool(v)
		},
		reflect.Int: func(v string) (interface{}, error) {
			return strconv.Atoi(v)
		},
	}
)

type envVarType interface {
	~string | ~bool | ~int
}

func parseEnvVar[T envVarType](envVar string, envValue string) T {
	var envParsedValue T
	var err error

	envVarType := reflect.TypeOf(envParsedValue)
	value, err := defaultBuiltInParsers[envVarType.Kind()](envValue)
	if err != nil {
		log.Fatalf("%s environment variable is not valid: '%s' cannot be converted to %T", envVar, envValue, envParsedValue)
	}
	return value.(T)
}

func GetEnv[T envVarType](envVar string, defaultValue T) T {
	envValue, ok := os.LookupEnv(envVar)
	if !ok || envValue == "" {
		log.Printf("%s environment variable is not set, using default value '%v'", envVar, defaultValue)
		return defaultValue
	}
	return parseEnvVar[T](envVar, envValue)
}

func GetRequiredEnv[T envVarType](envVar string) T {
	envValue, ok := os.LookupEnv(envVar)
	if !ok || envValue == "" {
		log.Fatalf("%s environment variable is required", envVar)
	}
	return parseEnvVar[T](envVar, envValue)
}
