package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// 登录状态
var isLogin = false
var wsConn *websocket.Conn

// LoginMessage 登录消息结构
type LoginMessage struct {
	Action  string  `json:"action"`
	Token   string  `json:"token"`
	Version float64 `json:"version"` // 客户端版本
}

// ResponseMessage 响应消息结构
type ResponseMessage struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Version float64         `json:"version"`
}

var reportData struct {
	Host  Host      `json:"Host"`  // 使用指针类型
	State HostState `json:"State"` // 使用指针类型
}

// ConnectToServer 连接到 WebSocket 服务端
func ConnectToServer(url string, token string) error {
	attempt := 0 // 重连次数
	ua := fmt.Sprintf("LightMonitorClient/%s", fmt.Sprintf("%.1f", version))

	for {
		attempt++

		// 计算重连间隔
		var waitTime time.Duration
		switch {
		case attempt <= 5:
			waitTime = 100 * time.Millisecond
		case attempt <= 30:
			waitTime = 1 * time.Second
		default:
			waitTime = 5 * time.Second
		}

		// 只在第一次尝试重连时打印日志
		if attempt == 1 {
			log.Println("正在连接...")
		}

		// 延迟重连时间
		if attempt > 1 {
			time.Sleep(waitTime)
		}

		// 设置 WebSocket 请求头
		header := http.Header{}
		header.Set("User-Agent", ua)

		// 尝试建立 WebSocket 连接
		conn, _, err := websocket.DefaultDialer.Dial(url, header)
		if err != nil {
			continue
		}

		wsConn = conn

		// 初始化状态
		isLogin = false

		// 开始处理 WebSocket 消息
		if err, code := handleConnection(conn, token); err != nil {
			if code == 2 {
				os.Exit(1)
			}
			log.Printf("连接已关闭")
			continue
		}
	}
}

// handleConnection 处理 WebSocket 消息
func handleConnection(conn *websocket.Conn, token string) (error, int) {
	defer conn.Close()

	code := -1

	// 接收欢迎消息
	if err := receiveMessage(conn, func(response ResponseMessage) error {
		if response.Status == 0 {
			log.Printf("%s 服务正常 v%s\n", response.Message, fmt.Sprintf("%.1f", response.Version))
		}
		return nil
	}); err != nil {
		return err, -1
	}

	// 发送登录信息
	loginMsg := LoginMessage{
		Action:  "login",
		Token:   token,
		Version: version,
	}
	if err := sendMessage(conn, loginMsg); err != nil {
		return err, -1
	}

	// 接收登录响应
	if err := receiveMessage(conn, func(response ResponseMessage) error {
		// 解析 data
		var data struct {
			Name   string `json:"name"`
			Region string `json:"region"`
			City   string `json:"city"`
		}
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return fmt.Errorf("登录超时")
		}
		log.Printf("登录成功！名称: %s, 地区: %s, 城市: %s\n", data.Name, data.Region, data.City)
		isLogin = true
		return nil
	}); err != nil {
		return err, code
	}

	// 开始接收消息
	//log.Println("等待服务端消息...")
	for {
		if err := receiveMessage(conn, func(response ResponseMessage) error {
			return nil
		}); err != nil {
			//log.Printf("连接断开\n")
			break
		}
	}

	return nil, -1
}

// receiveMessage 接收并处理消息
func receiveMessage(conn *websocket.Conn, handler func(response ResponseMessage) error) error {
	_, message, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	//log.Printf(string(message))

	// 解压和解析消息
	var response ResponseMessage
	if err := parseMessage(message, &response); err != nil {
		return err
	}
	switch response.Status {
	case 2: // 非法请求
		log.Printf(response.Message)
		os.Exit(2)
	case 3: // 非法请求
		log.Printf(response.Message)
		os.Exit(2)
	case 4: // 客户端版本过低
		log.Printf(response.Message)
		os.Exit(2)
	default:
		return handler(response)
	}
	return err
}

// sendMessage 发送消息
func sendMessage(conn *websocket.Conn, data interface{}) error {
	message, err := json.Marshal(data)
	if err != nil {
		//return fmt.Errorf("序列化消息失败: %v", err)
	}
	return conn.WriteMessage(websocket.TextMessage, message)
}

// parseMessage 解压并解析 WebSocket 消息
func parseMessage(data []byte, v interface{}) error {
	reader := bytes.NewReader(data)
	gz, err := gzip.NewReader(reader)
	if err == nil {
		defer gz.Close()
		data, err = ioutil.ReadAll(gz)
		if err != nil {
			return err
		}
	}

	return json.Unmarshal(data, v)
}

// NodeReport 定时发送报告
func NodeReport() {
	ticker := time.NewTicker(1 * time.Second) // 每秒发送一次
	defer ticker.Stop()

	for range ticker.C {
		// 检查登录状态
		if !isLogin {
			time.Sleep(time.Second) // 等待 1 秒再检查
			continue
		}

		// 获取当前 WebSocket 连接
		conn := wsConn
		if conn == nil {
			time.Sleep(time.Second) // 等待 1 秒再检查
			continue
		}

		// 获取最新的 Host 和 State 信息
		hostStateInfo, err := GetHostStateInfo()
		if err != nil {
			//log.Printf("获取主机状态信息失败: %v", err)
			continue
		}

		// 当前主机信息
		currentHost := hostStateInfo.Host

		// 当前主机状态信息
		state := hostStateInfo.State

		// 判断 Host 是否发生变化
		shouldSendHost := false
		if !compareHosts(lastHost, currentHost) {
			shouldSendHost = true
			lastHost = currentHost // 更新 lastHost
		}

		// 构建报告数据
		reportData := map[string]interface{}{
			"State": state, // 始终包括 State
		}

		// 如果 Host 发生变化，添加 Host 字段
		if shouldSendHost {
			reportData["Host"] = currentHost
		}

		// 序列化为 JSON
		reportMessage := struct {
			Action string      `json:"action"`
			Data   interface{} `json:"data"`
		}{
			Action: "report",
			Data:   reportData,
		}

		// 发送报告
		if err := sendMessage(conn, reportMessage); err != nil {
			isLogin = false
		}
	}
}
