package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type MarkNotificationsReadRequest struct {
	AnnouncementKeys []string `json:"announcement_keys"`
}

func GetNotificationReadState(c *gin.Context) {
	keys, err := model.ListUserAnnouncementReadKeys(c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"announcement_keys": keys,
		},
	})
}

func MarkNotificationsRead(c *gin.Context) {
	var req MarkNotificationsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	keys, err := model.MarkUserAnnouncementsRead(c.GetInt("id"), req.AnnouncementKeys)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"announcement_keys": keys,
		},
	})
}
