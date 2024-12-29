package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

// Config 结构体定义
type Config struct {
	Listen     string         `yaml:"listen"`
	Token      string         `yaml:"token"`
	NodeURI    string         `yaml:"node_uri"`
	BroadURI   string         `yaml:"broad_uri"`
	ConsoleURI string         `yaml:"console_uri"`
	Database   DatabaseConfig `yaml:"database"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`
	FilePath string `yaml:"filepath"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// Global Config variable
var config Config

// LoadConfig 加载配置文件或从命令行获取参数
func LoadConfig() error {
	// 定义命令行参数
	configFile := flag.String("c", "Server.yaml", "配置文件路径 (默认为当前程序目录下的 Server.yaml)")
	token := flag.String("token", "", "Token")
	listen := flag.String("listen", ":80", "监听端口")
	nodeUri := flag.String("node_uri", "/Monitor/Node", "节点 URI")
	broadUri := flag.String("broad_uri", "/Monitor/Status", "广播 URI")
	consoleUri := flag.String("console_uri", "/Monitor/Console", "控制台 URI")
	dbType := flag.String("type", "sqlite", "数据库类型")
	sqlitePath := flag.String("sqlite_path", "LightMonitor.db", "数据库文件路径")
	host := flag.String("host", "127.0.0.1", "数据库主机")
	port := flag.Int("port", 3306, "数据库端口")
	user := flag.String("user", "", "数据库用户名")
	password := flag.String("password", "", "数据库密码")
	dbname := flag.String("dbname", "LightMonitor", "数据库名称")
	flag.Parse()

	// 是否使用命令行参数中的配置
	if *token != "" {
		config.Token = *token
	}

	if *listen != "" {
		config.Listen = *listen
	}

	if *nodeUri != "" {
		config.NodeURI = *nodeUri
	}

	if *broadUri != "" {
		config.BroadURI = *broadUri
	}

	if *consoleUri != "" {
		config.ConsoleURI = *consoleUri
	}

	// 数据库配置
	config.Database = DatabaseConfig{
		Type:     *dbType,
		FilePath: *sqlitePath,
		Host:     *host,
		Port:     *port,
		User:     *user,
		Password: *password,
		DBName:   *dbname,
	}

	if config.Token != "" {
		return nil
	}

	// 则加载配置文件
	if *configFile == "" {
		currentDir := getCurrentDir()
		*configFile = filepath.Join(currentDir, "Server.yaml")
	}

	if err := loadConfigFromFile(*configFile); err != nil {
		return err
	}

	// 检查数据库配置是否正确
	if err := validateDatabaseConfig(); err != nil {
		return err
	}
	return nil
}

// loadConfigFromFile 从配置文件加载配置
func loadConfigFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	return nil
}

// validateDatabaseConfig 校验数据库配置是否正确
func validateDatabaseConfig() error {
	if config.Database.Type == "" {
		return fmt.Errorf("数据库类型不能为空")
	}

	if config.Database.Type == "sqlite" {
		// sqlite需要文件路径配置
		if config.Database.FilePath == "" {
			return fmt.Errorf("sqlite数据库需要提供文件路径")
		}
	} else if config.Database.Type == "mysql" {
		// MySQL需要 host, port, user, password 配置
		if config.Database.Host == "" || config.Database.Port == 0 || config.Database.User == "" || config.Database.Password == "" {
			return fmt.Errorf("mysql数据库需要提供 host, port, user, password")
		}
	} else {
		return fmt.Errorf("不支持的数据库类型: %s", config.Database.Type)
	}
	return nil
}

// getCurrentDir 获取当前程序所在目录
func getCurrentDir() string {
	ex, err := os.Executable()
	if err != nil {
		fmt.Println("获取程序路径失败:", err)
		os.Exit(1)
	}
	dir := filepath.Dir(ex)
	return dir
}
