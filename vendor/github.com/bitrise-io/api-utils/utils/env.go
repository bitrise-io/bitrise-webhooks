package utils

import (
	"os"
	"strconv"
)

// GetInt64EnvWithDefault ...
func GetInt64EnvWithDefault(key string, defaultValue int64) int64 {
	value, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil {
		value = defaultValue
	}
	return value
}
