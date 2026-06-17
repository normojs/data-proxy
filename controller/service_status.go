package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	servicestatus "github.com/QuantumNous/new-api/pkg/service_status"

	"github.com/gin-gonic/gin"
)

func GetServiceStatusSummary(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	result, err := servicestatus.Query(servicestatus.QueryOptions{
		Hours:         hours,
		IncludeAlerts: c.GetInt("role") >= common.RoleAdminUser,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
