package controller

import (
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

var (
	setupRuntimeRestartScheduled atomic.Bool
	setupRuntimeRestartSupported = common.IsRunningInContainer
	scheduleSetupRuntimeRestart  = func(delay time.Duration) {
		go func() {
			time.Sleep(delay)
			common.SysLog("exiting Data Proxy to apply setup runtime config")
			os.Exit(0)
		}()
	}
)

type Setup struct {
	Status                       bool   `json:"status"`
	RootInit                     bool   `json:"root_init"`
	DatabaseType                 string `json:"database_type"`
	DatabaseConfigured           bool   `json:"database_configured"`
	DatabaseSource               string `json:"database_source"`
	RedisEnabled                 bool   `json:"redis_enabled"`
	RedisConfigured              bool   `json:"redis_configured"`
	RedisSource                  string `json:"redis_source"`
	RuntimeConfigLoaded          bool   `json:"runtime_config_loaded"`
	RuntimeConfigRestartRequired bool   `json:"runtime_config_restart_required"`
	RuntimeConfigPath            string `json:"runtime_config_path"`
}

type SetupRequest struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	ConfirmPassword    string `json:"confirmPassword"`
	SelfUseModeEnabled bool   `json:"SelfUseModeEnabled"`
	DemoSiteEnabled    bool   `json:"DemoSiteEnabled"`
}

type SetupRuntimeConfigRequest struct {
	DatabaseType    string `json:"database_type"`
	SQLDSN          string `json:"sql_dsn"`
	SQLitePath      string `json:"sqlite_path"`
	RedisEnabled    bool   `json:"redis_enabled"`
	RedisConnString string `json:"redis_conn_string"`
}

func GetSetup(c *gin.Context) {
	setup := currentSetupStatus()
	if constant.Setup {
		c.JSON(200, gin.H{
			"success": true,
			"data":    setup,
		})
		return
	}
	setup.RootInit = model.RootUserExists()
	c.JSON(200, gin.H{
		"success": true,
		"data":    setup,
	})
}

func currentSetupStatus() Setup {
	setup := Setup{
		Status:                       constant.Setup,
		DatabaseConfigured:           os.Getenv("SQL_DSN") != "",
		DatabaseSource:               common.RuntimeConfigValueSource("SQL_DSN"),
		RedisEnabled:                 common.RedisEnabled,
		RedisConfigured:              os.Getenv("REDIS_CONN_STRING") != "",
		RedisSource:                  common.RuntimeConfigValueSource("REDIS_CONN_STRING"),
		RuntimeConfigLoaded:          common.RuntimeConfigLoaded,
		RuntimeConfigRestartRequired: common.RuntimeConfigRestartRequired,
		RuntimeConfigPath:            common.RuntimeConfigPath,
	}
	if common.UsingMySQL {
		setup.DatabaseType = "mysql"
	}
	if common.UsingPostgreSQL {
		setup.DatabaseType = "postgres"
	}
	if common.UsingSQLite {
		setup.DatabaseType = "sqlite"
	}
	if setup.DatabaseSource == "" {
		if setup.DatabaseConfigured {
			setup.DatabaseSource = "env"
		} else {
			setup.DatabaseSource = "sqlite-default"
		}
	}
	if setup.RedisSource == "" && setup.RedisConfigured {
		setup.RedisSource = "env"
	}
	return setup
}

func PostSetupRuntimeConfig(c *gin.Context) {
	if constant.Setup {
		c.JSON(200, gin.H{
			"success": false,
			"message": "系统已经初始化完成",
		})
		return
	}

	var req SetupRuntimeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "请求参数有误",
		})
		return
	}

	dbType := strings.ToLower(strings.TrimSpace(req.DatabaseType))
	sqlDSN := strings.TrimSpace(req.SQLDSN)
	sqlitePath := strings.TrimSpace(req.SQLitePath)
	switch dbType {
	case "", "sqlite":
		dbType = "sqlite"
		sqlDSN = "local"
	case "mysql":
		if sqlDSN == "" {
			c.JSON(200, gin.H{
				"success": false,
				"message": "请填写 MySQL 连接字符串",
			})
			return
		}
		if detected := model.DetectDatabaseType(sqlDSN); detected != "mysql" {
			c.JSON(200, gin.H{
				"success": false,
				"message": "当前连接字符串不是 MySQL 格式",
			})
			return
		}
	case "postgres", "postgresql":
		dbType = "postgres"
		if sqlDSN == "" {
			c.JSON(200, gin.H{
				"success": false,
				"message": "请填写 PostgreSQL 连接字符串",
			})
			return
		}
		if detected := model.DetectDatabaseType(sqlDSN); detected != "postgres" {
			c.JSON(200, gin.H{
				"success": false,
				"message": "当前连接字符串不是 PostgreSQL 格式",
			})
			return
		}
	default:
		c.JSON(200, gin.H{
			"success": false,
			"message": "不支持的数据库类型",
		})
		return
	}

	detectedType, err := model.TestDatabaseConnection(sqlDSN, sqlitePath)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "数据库连接测试失败: " + err.Error(),
		})
		return
	}
	if dbType != detectedType {
		c.JSON(200, gin.H{
			"success": false,
			"message": "数据库类型与连接字符串不匹配",
		})
		return
	}

	redisConnString := strings.TrimSpace(req.RedisConnString)
	if req.RedisEnabled {
		if redisConnString == "" {
			c.JSON(200, gin.H{
				"success": false,
				"message": "启用 Redis 时必须填写连接字符串",
			})
			return
		}
		if err := common.TestRedisConnection(redisConnString); err != nil {
			c.JSON(200, gin.H{
				"success": false,
				"message": "Redis 连接测试失败: " + err.Error(),
			})
			return
		}
	}

	if err := common.SaveRuntimeConfig(common.RuntimeConfig{
		SQLDSN:          sqlDSN,
		SQLitePath:      sqlitePath,
		RedisConnString: redisConnString,
	}); err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "保存运行配置失败: " + err.Error(),
		})
		return
	}

	restartScheduled := maybeScheduleSetupRuntimeRestart()
	message := "运行配置已保存，请重启 Data Proxy 后继续初始化"
	if restartScheduled {
		message = "运行配置已保存，Data Proxy 正在自动重启以应用配置"
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"database_type":     detectedType,
			"redis_configured":  redisConnString != "",
			"restart_required":  true,
			"restart_supported": setupRuntimeRestartSupported(),
			"restart_scheduled": restartScheduled,
			"restart_delay_ms":  1200,
		},
	})
}

func maybeScheduleSetupRuntimeRestart() bool {
	if strings.EqualFold(os.Getenv("DATA_PROXY_SETUP_AUTO_RESTART"), "false") {
		return false
	}
	if !setupRuntimeRestartSupported() {
		return false
	}
	if setupRuntimeRestartScheduled.CompareAndSwap(false, true) {
		scheduleSetupRuntimeRestart(1200 * time.Millisecond)
	}
	return true
}

func PostSetup(c *gin.Context) {
	// Check if setup is already completed
	if constant.Setup {
		c.JSON(200, gin.H{
			"success": false,
			"message": "系统已经初始化完成",
		})
		return
	}

	// Check if root user already exists
	rootExists := model.RootUserExists()

	var req SetupRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "请求参数有误",
		})
		return
	}

	// If root doesn't exist, validate and create admin account
	if !rootExists {
		// Validate username length: max 12 characters to align with model.User validation
		if len(req.Username) > 12 {
			c.JSON(200, gin.H{
				"success": false,
				"message": "用户名长度不能超过12个字符",
			})
			return
		}
		// Validate password
		if req.Password != req.ConfirmPassword {
			c.JSON(200, gin.H{
				"success": false,
				"message": "两次输入的密码不一致",
			})
			return
		}

		if len(req.Password) < 8 {
			c.JSON(200, gin.H{
				"success": false,
				"message": "密码长度至少为8个字符",
			})
			return
		}

		// Create root user
		hashedPassword, err := common.Password2Hash(req.Password)
		if err != nil {
			c.JSON(200, gin.H{
				"success": false,
				"message": "系统错误: " + err.Error(),
			})
			return
		}
		rootUser := model.User{
			Username:    req.Username,
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		err = model.DB.Create(&rootUser).Error
		if err != nil {
			c.JSON(200, gin.H{
				"success": false,
				"message": "创建管理员账号失败: " + err.Error(),
			})
			return
		}
	}

	// Set operation modes
	operation_setting.SelfUseModeEnabled = req.SelfUseModeEnabled
	operation_setting.DemoSiteEnabled = req.DemoSiteEnabled

	// Save operation modes to database for persistence
	err = model.UpdateOption("SelfUseModeEnabled", boolToString(req.SelfUseModeEnabled))
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "保存自用模式设置失败: " + err.Error(),
		})
		return
	}

	err = model.UpdateOption("DemoSiteEnabled", boolToString(req.DemoSiteEnabled))
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "保存演示站点模式设置失败: " + err.Error(),
		})
		return
	}

	// Update setup status
	constant.Setup = true

	setup := model.Setup{
		Version:       common.Version,
		InitializedAt: time.Now().Unix(),
	}
	err = model.DB.Create(&setup).Error
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": "系统初始化失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "系统初始化成功",
	})
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
