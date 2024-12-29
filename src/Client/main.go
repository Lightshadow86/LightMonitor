package main

import (
	"fmt"
	"log"
	"os"
)

var version float64

func main() {
	version = 0.1
	// 加载配置文件

	log.Printf("LightMonitor v%s\n", fmt.Sprintf("%.1f", version))
	log.Printf("\n")
	config, err := LoadConfig()
	if err != nil {
		log.Printf("    -c      指定配置文件路径\n\n")
		log.Printf("    -url    指定API URL路径|ws(s)://api.example.com/Monitor/Node\n")
		log.Printf("    -token  指定节点Token\n\n")
		log.Printf("    当url和token同时存在时，忽略配置文件")
		return
	}

	go NodeReport()

	// 尝试连接 WebSocket 并登录
	log.Printf("正在连接到 %s\n", config.URL)
	if err := ConnectToServer(config.URL, config.Token); err != nil {
		os.Exit(1)
	}

	//log.Println("等待服务端消息...")
}
