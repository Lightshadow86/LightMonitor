package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

var version float64

func main() {
	version = 0.1
	// 加载配置
	log.Printf("LightMonitor-Server v%s\n", fmt.Sprintf("%.1f", version))
	log.Printf("\n")
	err := LoadConfig()
	if err != nil {
		log.Printf("用法： \n")
		log.Printf("    -c          	指定配置文件路径 (默认为当前程序目录下的 Server.yaml)\n")
		log.Printf("    -listen     	指定监听端口 (默认为 :80)\n")
		log.Printf("    -node_uri   	指定Node API路径 (默认为 /Monitor/Node)\n")
		log.Printf("    -broad_uri  	指定广播API路径 (默认为 /Monitor/Status)\n")
		log.Printf("    -console_uri	指定控制台API路径 (默认为 /Monitor/Console)\n")
		log.Printf("    -token      	指定节点Token\n")
		log.Printf("    -type       	指定数据库类型 (默认为 sqlite)\n")
		log.Printf("    -filepath   	指定数据库文件路径 (默认为 LightMonitor.db)\n")
		log.Printf("    -host       	指定数据库主机 (默认为 127.0.0.1)\n")
		log.Printf("    -port       	指定数据库端口 (默认为 3306)\n")
		log.Printf("    -user       	指定数据库用户名\n")
		log.Printf("    -password   	指定数据库密码\n")
		log.Printf("    -dbname     	指定数据库名称 (默认为 LightMonitor)\n")
		log.Printf("\n")
		log.Printf("    当 token 存在时，忽略配置文件\n")
		os.Exit(1)
	}

	// 初始化数据库
	err = InitDatabase(config.Database)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	log.Printf("监听地址: %s\n", config.Listen)
	log.Printf("数据库类型: %s", config.Database.Type)
	if config.Database.Type == "sqlite" {
		log.Printf("数据库地址: %s", config.Database.FilePath)
	}
	log.Printf("节点 URI: %s\n", config.NodeURI)
	log.Printf("广播 URI: %s\n", config.BroadURI)

	// 初始化 WebSocket 路由
	initRoutes()

	// 启动广播的 Goroutine
	go FetchData()
	go StartBroad()

	// 启动 HTTP 服务
	err = http.ListenAndServe(config.Listen, nil)
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
