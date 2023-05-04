package utils

import (
	"log"
	"os"
	"strconv"
)

func GetRequiredStringEnv(envVar string) string {
	envValue, ok := os.LookupEnv(envVar)
	if !ok || envValue == "" {
		log.Fatalf("set %s environment variable", envVar)
	}
	return envValue
}

func GetStringEnv(envVar string, defaultValue string) string {
	envValue, ok := os.LookupEnv(envVar)
	if !ok || envValue == "" {
		log.Printf("no %s environment variable (default to %s)", envVar, defaultValue)
		return defaultValue
	}
	return envValue
}

func GetRequiredBoolEnv(envVar string) bool {
	envValue, err := strconv.ParseBool(GetRequiredStringEnv(envVar))
	if err != nil {
		log.Fatalf("set %s environment variable: %s", envVar, err)
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
		log.Fatalf("set %s environment variable: %s", envVar, err)
	}
	return envValue
}
