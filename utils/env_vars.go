package utils

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

func GetIntEnv(envVarName string, defaultValue int) int {
	envValue, ok := os.LookupEnv(envVarName)
	if !ok || envValue == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(envValue)
	if err != nil {
		panic(fmt.Sprintf("Environment variable %s is not valid. '%s' is not an integer", envVarName, envValue))
	}
	return intValue
}

func GetRequiredStringEnv(envVar string) string {
	envValue, ok := os.LookupEnv(envVar)
	if !ok || envValue == "" {
		log.Fatalf("%s environment variable is required", envVar)
	}
	return envValue
}

func GetStringEnv(envVar string, defaultValue string) string {
	envValue, ok := os.LookupEnv(envVar)
	if !ok || envValue == "" {
		log.Printf("%s environment variable is not set, using default value '%s'", envVar, defaultValue)
		return defaultValue
	}
	return envValue
}

func GetRequiredBoolEnv(envVar string) bool {
	envValue, err := strconv.ParseBool(GetRequiredStringEnv(envVar))
	if err != nil {
		log.Fatalf("%s environment variable is no valid. '%s' cannot be converted to bool", envVar, err)
	}
	return envValue
}

func GetBoolEnv(envVar string, defaultValue bool) bool {
	stringEnvValue := GetStringEnv(envVar, "")
	if stringEnvValue == "" {
		return defaultValue
	}
	envValue, err := strconv.ParseBool(stringEnvValue)
	if err != nil {
		log.Fatalf("%s environment variable is not valid: '%s' cannot be converted to bool", envVar, err)
	}
	return envValue
}
