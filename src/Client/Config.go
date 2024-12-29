package main

import (
	"flag"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Config struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

// LoadConfig 从配置文件加载配置
func LoadConfig() (*Config, error) {
	configPath := "Client.yaml"

	// 定义命令行参数
	configFile := flag.String("c", "", "配置文件路径 (默认为当前程序目录下的 Client.yaml)")
	url := flag.String("url", "", "ws(s)://api.example.com/Monitor/Node")
	token := flag.String("token", "", "Token")
	flag.Parse()

	var config Config

	// 判断是否同时提供了 -url 和 -token
	if *url != "" && *token != "" {
		//fmt.Println("使用命令行参数 URL 和 Token")
		//fmt.Printf("URL: %s, Token: %s\n", *url, *token)
		config.URL = *url
		config.Token = *token
		return &config, nil
	}

	// 如果没有提供 -c 参数，则尝试默认加载配置文件
	if *configFile == "" {
		// 获取当前程序所在目录
		currentDir := getCurrentDir()
		*configFile = filepath.Join(currentDir, "Client.yaml")
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// 获取当前程序的路径
func getCurrentDir() string {
	// 获取当前程序所在的路径
	ex, err := os.Executable()
	if err != nil {
		//fmt.Println("获取程序路径失败:", err)
		os.Exit(1)
	}
	dir := filepath.Dir(ex)
	return dir
}
