package controller

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

func parseOptionalIntQuery(c *gin.Context, name string) (int, error) {
	value := c.Query(name)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func parseOptionalInt64Query(c *gin.Context, name string) (int64, error) {
	value := c.Query(name)
	if value == "" {
		return 0, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func parseIntPathParam(c *gin.Context, name string, label string) (int, error) {
	value := c.Param(name)
	if value == "" {
		if label == "" {
			label = name
		}
		return 0, fmt.Errorf("%s is required", label)
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		if label == "" {
			label = name
		}
		return 0, fmt.Errorf("invalid %s", label)
	}
	return id, nil
}
