package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//这玩意太难写了，这是最主要的部分也是最难的一个部分，尤其是对于我这个接触没几天的人来说

// WSUpgrade WebSocket 升级器
var WSUpgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有连接（可以根据需求修改为严格的检查规则）
		return true
	},
}

var WSConnections = make(map[string]map[string]interface{}) // 存储所有连接的客户端
var activeMutex sync.Mutex                                  // 连接锁

// GetGzip 封装或解压数据
func GetGzip(data []byte, compress bool) ([]byte, error) {
	var buffer bytes.Buffer
	if compress {
		// 创建一个gzip压缩写入器
		writer := gzip.NewWriter(&buffer)
		// 将数据写入压缩器
		_, err := writer.Write(data)
		if err != nil {
			return nil, fmt.Errorf("gzip压缩错误: %w", err)
		}
		// 关闭压缩器，确保所有数据都被写入
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("gzip写入器关闭错误: %w", err)
		}
	} else {
		// 创建一个gzip解压读取器
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip读取器创建错误: %w", err)
		}
		// 将解压后的数据复制到buffer中
		_, err = io.Copy(&buffer, reader)
		if err != nil {
			return nil, fmt.Errorf("gzip读取器复制错误: %w", err)
		}
		// 关闭解压读取器
		if err := reader.Close(); err != nil {
			return nil, fmt.Errorf("gzip读取器关闭错误: %w", err)
		}
	}
	// 返回buffer中的字节数组
	return buffer.Bytes(), nil
}

// BroadWS 处理广播 WebSocket 连接
func BroadWS(w http.ResponseWriter, r *http.Request) {
	var clientAddr, clientKey, clientUA, clientIPType, _ string

	// 升级到WS
	conn, err := WSUpgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v\n", err)
		return
	}

	// 获取客户端信息
	clientKey, clientAddr, clientIPType, clientUA, _ = ClientInfo(r)
	AddWSClient(clientKey, clientAddr, clientIPType, clientUA, "", "广播", conn)
	defer RemoveWSClient(clientKey)

	// 广播状态
	if !isBroad {
		isBroad = true // 至少有一个客户端连接时启用广播
		log.Println("前端广播已开始")
	}

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func NodeWS(w http.ResponseWriter, r *http.Request) {
	var clientAddr, clientKey, clientUA, clientIPType, clientEncoding string
	var NodeID int

	// 升级到WS
	conn, err := WSUpgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v\n", err)
		return
	}

	clientKey, clientAddr, clientIPType, clientUA, clientEncoding = ClientInfo(r)
	AddWSClient(clientKey, clientAddr, clientIPType, clientUA, clientEncoding, "节点", conn)
	defer RemoveWSClient(clientKey)

	// 发送欢迎信息
	welcomeMessage := map[string]interface{}{
		"status":  0,
		"message": "LightMonitor",
		"version": version,
	}

	message, err := json.Marshal(welcomeMessage)
	if err != nil {
		log.Printf("欢迎信息生成失败: %v", err)
		return
	}

	err = SendWS(conn, message, clientEncoding)
	if err != nil {
		log.Printf("欢迎信息发送失败: %v", err)
		return
	}

	for {
		messageType, messageData, err := conn.ReadMessage()

		// 断开连接
		if err != nil {
			//log.Printf("连接断开\n")
			break
		}

		if messageType == websocket.TextMessage {
			var received map[string]interface{}
			if clientEncoding == "gzip" {
				messageData, err = GetGzip(messageData, false)
				if err != nil {
					log.Printf("解码Gzip失败: %v\n", err)
					err = SendWS(conn, []byte("{\"status\":3,\"message\":\"gzip解压失败\"}"), clientEncoding)
					if err != nil {
						log.Printf("错误请求回应发送失败: %v", err)
						return
					}
					continue
				}
			}
			err = json.Unmarshal(messageData, &received)
			if err != nil {
				log.Printf("解码消息失败: %v\n", err)
				err = SendWS(conn, []byte("{\"status\":3,\"message\":\"json解码失败\"}"), clientEncoding)
				if err != nil {
					log.Printf("错误请求回应发送失败: %v", err)
					return
				}
				continue
			}

			// 判断行为逻辑
			if action, ok := received["action"]; ok {
				switch action {
				// 处理登录
				case "login":
					tokenRaw, exists := received["token"]
					if !exists {
						log.Printf("Token不存在: \n")
						err = SendWS(conn, []byte("{\"status\":3,\"message\":\"Token不存在\"}"), clientEncoding)
						if err != nil {
							log.Printf("错误请求回应发送失败: %v", err)
							return
						}
						continue
					}

					// 处理Token
					var token string
					switch t := tokenRaw.(type) {
					case string:
						token = t
					case float64:
						token = strconv.FormatFloat(t, 'f', -1, 64)
					default:
						log.Printf("token 字段类型解析失败！内容: %v\n", tokenRaw)
						err = SendWS(conn, []byte("{\"status\":3,\"message\":\"Token类型解析失败\"}"), clientEncoding)
						if err != nil {
							log.Printf("错误请求回应发送失败: %v", err)
							return
						}
						continue
					}

					// 处理登录
					err, NodeID = Login(conn, clientKey, token, clientAddr, clientEncoding)
					if err != nil {
						log.Printf("登录失败: %v\n", err)
						break
					}
				// 处理上报
				case "report":
					// 提取数据
					data, exists := received["data"].(map[string]interface{})
					if !exists {
						log.Printf("report 数据缺失或格式错误: %v\n", received)
						err := SendWS(conn, []byte(`{"status":3,"message":"非法请求"}`), clientEncoding)
						if err != nil {
							return
						}
					}

					// 处理数据
					err = GetData(NodeID, data)
					if err != nil {
						log.Printf("处理 report 数据失败: %v\n", err)
						err := SendWS(conn, []byte(`{"status":3,"message":"服务器内部错误"}`), clientEncoding)
						if err != nil {
							return
						}
					}

					// 上报成功
					err = SendWS(conn, []byte(`{"status":1}`), clientEncoding)
					if err != nil {
						continue
					}

				// 非法请求
				default:
					err := SendWS(conn, []byte(`{"status":3,"message":"非法请求"}`), clientEncoding)
					if err != nil {
						return
					}
				}
			} else {
				err := SendWS(conn, []byte(`{"status":3,"message":"非法请求"}`), clientEncoding)
				if err != nil {
					return
				}
			}
		}
	}
}

// SendWS 发送websocket消息，支持gzip压缩
func SendWS(conn *websocket.Conn, message []byte, clientEncoding string) error {
	var err error
	switch clientEncoding {
	case "gzip":
		// 使用gzip压缩消息
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		_, err = writer.Write(message)
		if err != nil {
			return fmt.Errorf("gzip压缩错误: %w", err)
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("gzip写入器关闭错误: %w", err)
		}
		err = conn.WriteMessage(websocket.TextMessage, buf.Bytes())
		if err != nil {
			return fmt.Errorf("websocket发送消息错误: %w", err)
		}
	default:
		// 不使用压缩，直接发送消息
		err = conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return fmt.Errorf("websocket发送消息错误: %w", err)
		}
	}

	return err
}

// StartBroad 广播数据到所有连接的客户端
func StartBroad() {
	for {
		time.Sleep(1 * time.Second)
		activeMutex.Lock()
		if !isBroad || BroadData == "" {
			activeMutex.Unlock()
			continue
		}

		for _, clientInfo := range WSConnections {
			if clientInfo["Type"] == "广播" {
				wsConn, ok := clientInfo["Conn"].(*websocket.Conn)
				if !ok {
					log.Printf("广播连接WebSocket信息提取失败")
					continue
				}

				err := SendWS(wsConn, []byte(BroadData), "")
				if err != nil {
					log.Printf("广播发送失败 %v", err)
					continue
				}
			}
		}
		activeMutex.Unlock()
	}
}

// Login 用户登录函数
func Login(conn *websocket.Conn, clientKey, token, NodeIP, clientEncoding string) (error, int) {
	var nodeID int
	var name, region, city string
	var nameStr, regionStr, cityStr *string

	rows, err := SQLRead("SELECT ID, Name, Region, City FROM Node WHERE Token = ?", token)
	if err != nil {
		err := SendWS(conn, []byte(`{"status":3,"message":"服务器内部错误"}`), clientEncoding)
		if err != nil {
			return err, nodeID
			rows.Close()
			dbMutex.RUnlock()
		}
	}

	if rows.Next() {
		err := rows.Scan(&nodeID, &nameStr, &regionStr, &cityStr)
		if err != nil {
			log.Printf("读取节点信息失败: %v", err)
			err := SendWS(conn, []byte(`{"status":3,"message":"服务器内部错误"}`), clientEncoding)
			if err != nil {
				rows.Close()
				dbMutex.RUnlock()
				return err, nodeID
			}
			return fmt.Errorf("读取节点信息失败: %w", err), nodeID
		}
	} else {
		log.Printf("%s Token无效", NodeIP)
		rows.Close()
		dbMutex.RUnlock()
		return SendWS(conn, []byte(`{"status":2,"message":"无效Token"}`), clientEncoding), nodeID
	}
	rows.Close()
	dbMutex.RUnlock()

	name = ""
	region = ""
	city = ""

	if nameStr != nil {
		name = *nameStr
	}
	if regionStr != nil {
		region = *regionStr
	}
	if cityStr != nil {
		city = *cityStr
	}

	err = SQLWrite("UPDATE Node SET IP = ? WHERE ID = ?", NodeIP, nodeID)
	if err != nil {
		log.Printf("%s 更新 Node IP 失败，UID: %s, 错误: %v\n", NodeIP, clientKey, err)
		err := SendWS(conn, []byte(`{"status":3,"message":"服务器内部错误"}`), clientEncoding)
		if err != nil {
			return err, nodeID
		}
		return fmt.Errorf("更新节点IP失败: %w", err), nodeID
	}

	err = SQLWrite("UPDATE Client SET ID = ? WHERE UID = ?", nodeID, clientKey)
	if err != nil {
		log.Printf("更新 Client ID 失败，UID: %s, 错误: %v\n", clientKey, err)
		err := SendWS(conn, []byte(`{"status":3,"message":"服务器内部错误"}`), clientEncoding)
		if err != nil {
			return err, nodeID
		}
		return fmt.Errorf("更新客户端ID失败: %w", err), nodeID
	}

	response := map[string]interface{}{
		"status":  1,
		"message": "登录成功！",
		"data": map[string]string{
			"name":   name,
			"region": region,
			"city":   city,
		},
	}

	// 为WebSocket连接添加登录信息
	activeMutex.Lock()
	WSConnections[clientKey]["name"] = name
	activeMutex.Unlock()

	log.Printf("%s 登录成功！名称: %s, 地区: %s, 城市: %s\n", NodeIP, name, region, city)
	responseData, err := json.Marshal(response)
	if err != nil {
		log.Printf("序列化登录响应失败: %v", err)
		err := SendWS(conn, []byte(`{"status":3,"message":"登录成功，但服务器内部错误"}`), clientEncoding)
		if err != nil {
			return err, nodeID
		}
		return fmt.Errorf("序列化登录响应失败: %w", err), nodeID
	}

	return SendWS(conn, responseData, clientEncoding), nodeID
}

// AddWSClient 添加 WebSocket 连接到池中
func AddWSClient(clientKey, clientAddr, clientIPType, clientUA, clientEncoding, clientType string, conn *websocket.Conn) bool {
	timestamp := time.Now().Unix()
	activeMutex.Lock()
	WSConnections[clientKey] = map[string]interface{}{
		"Conn":     conn,
		"Name":     "",
		"IP":       clientAddr,
		"IPType":   clientIPType,
		"UA":       clientUA,
		"Encoding": clientEncoding,
		"Type":     clientType,
	}
	activeMutex.Unlock()
	err := SQLWrite(`	INSERT INTO Client (UID, IP, IPType, Type, UA, TimeStamp)	VALUES (?, ?, ?, ?, ?, ?);	`,
		clientKey, clientAddr, clientIPType, clientType, clientUA, timestamp)
	if err != nil {
		log.Printf("插入客户端记录失败: %v", err)
		return false
	}

	log.Printf("%s %s 已连接\n", clientAddr, clientType)

	return true
}

func CheckBroad() {
	rows, err := SQLRead(`SELECT UID FROM Client WHERE Type == '广播' LIMIT 1`)
	if err != nil {
		log.Printf("检查是否需要广播错误: %v", err)
		rows.Close()
		dbMutex.RUnlock()
		return
	}

	if rows.Next() && isBroad {
		isBroad = false
		log.Println("前端广播已停止")
	}
	rows.Close()
	dbMutex.RUnlock()
}

// RemoveWSClient 从连接池中删除客户端
func RemoveWSClient(clientKey string) {
	var clientType interface{}
	activeMutex.Lock()
	defer activeMutex.Unlock()

	// 从连接池中移除客户端连接并关闭连接
	connMap, exists := WSConnections[clientKey]

	if exists {
		if conn, ok := connMap["Conn"].(*websocket.Conn); ok {
			conn.Close()
		}
		clientType = WSConnections[clientKey]["Type"]
		delete(WSConnections, clientKey)

		var ip, ua string
		// 查询客户端IP和UA信息
		rows, err := SQLRead("SELECT IP, UA FROM Client WHERE UID = ?", clientKey)
		if err != nil {
			log.Printf("查询客户端信息失败: %v\n", err)
			rows.Close()
			dbMutex.RUnlock()
			return
		}

		if rows.Next() {
			err := rows.Scan(&ip, &ua)
			if err != nil {
				log.Printf("扫描客户端信息失败: %v\n", err)
				rows.Close()
				dbMutex.RUnlock()
				return
			}
		} else {
			log.Printf("未找到UID对应的客户端信息: %s", clientKey)
			rows.Close()
			dbMutex.RUnlock()
			return
		}
		rows.Close()
		dbMutex.RUnlock()

		if clientType == "广播" {
			CheckBroad()
		}

		// 从数据库中删除客户端信息
		//err = SQLWrite("DELETE FROM Client WHERE UID = ?", clientKey)
		//if err != nil {
		//	log.Printf("数据库删除操作失败: %v\n", err)
		//}

		log.Printf("%s %s 已断开", ip, clientType)
	}
}

// SendToClient 向指定客户端发送消息
func SendToClient(clientKey, message string) error {
	activeMutex.Lock()
	defer activeMutex.Unlock()

	if connMap, ok := WSConnections[clientKey]; ok {
		if conn, ok := connMap["Conn"].(*websocket.Conn); ok {
			clientEncoding, ok := connMap["Encoding"].(string)
			if !ok {
				clientEncoding = ""
			}

			return SendWS(conn, []byte(message), clientEncoding)
		}
	}
	return fmt.Errorf("找不到客户端连接: %s", clientKey)
}

// KickClient 删除节点时检查客户端并发送消息或断开连接
func KickClient(ID int) {
	rows, err := SQLRead("SELECT UID FROM Client WHERE ID = ?", ID)
	if err != nil {
		log.Printf("查询客户端失败: %v\n", err)
		return
	}

	for rows.Next() {
		var clientKey string
		if err := rows.Scan(&clientKey); err != nil {
			log.Printf("扫描客户端UID失败: %v", err)
			continue
		}

		err := SendToClient(clientKey, `{"status":2, "message":"你已被删除"}`)
		if err != nil {
			log.Printf("向客户端 %s 发送消息失败: %v\n", clientKey, err)
		}
		RemoveWSClient(clientKey)
		//log.Printf("客户端 %s 由于被删除已踢出\n", clientKey)
	}
	rows.Close()
	dbMutex.RUnlock()
}

// Console 处理所有在 config.BroadURI 路径下的 HTTP POST 请求
func Console(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "*")

	_, ip, _, ua, _ := ClientInfo(r) // 获取客户端信息

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodPost {
		var requestData map[string]interface{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&requestData); err != nil {
			logMessage := fmt.Sprintf("%s 请求解析失败 | %s", ip, ua)
			log.Printf(logMessage)
			http.Error(w, fmt.Sprintf("请求解析失败: %v", err), http.StatusBadRequest)
			return
		}

		Token, ok := requestData["Token"].(string)
		if !ok {
			logMessage := fmt.Sprintf("%s 未携带密钥 | %s", ip, ua)
			log.Printf(logMessage)
			http.Error(w, "未携带密钥", http.StatusUnauthorized)
			return
		}

		if Token != config.Token {
			logMessage := fmt.Sprintf("%s 密钥不正确 | %s", ip, ua)
			log.Printf(logMessage)
			http.Error(w, fmt.Sprintf("密钥不正确"), http.StatusUnauthorized)
			return
		}

		action, ok := requestData["Action"].(string)
		if !ok {
			logMessage := fmt.Sprintf("%s 请求不合法 | %s", ip, ua)
			log.Printf(logMessage)
			http.Error(w, "请求不正确", http.StatusBadRequest)
			return
		}

		switch action {
		case "Add":
			name := requestData["Name"].(string)
			token := requestData["NodeToken"].(string)
			region := requestData["Region"].(string)
			city := requestData["City"].(string)

			var notAdd bool
			notAdd = false

			// 检查节点是否已存在
			rows, err := SQLRead("SELECT ID FROM Node WHERE Name = ?", name)
			if err != nil {
				logMessage := fmt.Sprintf("%s 节点 %s 更新失败，内部错误：数据库查询失败 | %s", ip, name, ua)
				log.Printf(logMessage)
				http.Error(w, fmt.Sprintf("内部错误：数据库查询失败"), http.StatusInternalServerError)
				rows.Close()
				dbMutex.RUnlock()
				return
			}

			if rows.Next() {
				var nodeID int
				if err := rows.Scan(&nodeID); err != nil {
					logMessage := fmt.Sprintf("%s 节点 %s 更新失败，内部错误：检查节点失败 | %s", ip, name, ua)
					log.Printf(logMessage)
					http.Error(w, fmt.Sprintf("内部错误：检查节点失败"), http.StatusInternalServerError)
					notAdd = true
					rows.Close()
					dbMutex.RUnlock()
					return

				}
				logMessage := fmt.Sprintf("%s 节点 %s 已存在，ID: %d，增加节点取消 | %s", ip, name, nodeID, ua)
				log.Printf(logMessage)
				http.Error(w, fmt.Sprintf("添加节点失败: 节点已存在，请不要使用相同的节点名称"), http.StatusInternalServerError)
				notAdd = true
				rows.Close()
				dbMutex.RUnlock()
				return
			}
			rows.Close()
			dbMutex.RUnlock()

			// 检查Token是否已存在
			rows, err = SQLRead("SELECT ID FROM Node WHERE Token = ?", token)
			if err != nil {
				logMessage := fmt.Sprintf("%s 节点 %s 更新失败，内部错误：数据库查询失败 | %s", ip, name, ua)
				log.Printf(logMessage)
				http.Error(w, fmt.Sprintf("内部错误：数据库查询失败"), http.StatusInternalServerError)
				notAdd = true
				rows.Close()
				dbMutex.RUnlock()
				return
			}

			if rows.Next() {
				var nodeID int
				if err := rows.Scan(&nodeID); err != nil {
					logMessage := fmt.Sprintf("%s 节点 %s 更新失败，内部错误：检查节点失败 | %s", ip, name, ua)
					log.Printf(logMessage)
					http.Error(w, fmt.Sprintf("内部错误：检查节点失败"), http.StatusInternalServerError)
					notAdd = true
					rows.Close()
					dbMutex.RUnlock()
					return

				}
				logMessage := fmt.Sprintf("%s Token %s 已存在，ID: %d，增加节点取消 | %s", ip, name, nodeID, ua)
				log.Printf(logMessage)
				http.Error(w, fmt.Sprintf("添加节点失败: Token已存在，请不要使用相同的Token"), http.StatusInternalServerError)
				notAdd = true
				rows.Close()
				dbMutex.RUnlock()
				return
			}
			rows.Close()
			dbMutex.RUnlock()

			if notAdd == false {
				err = AddNode(name, token, region, city)
				if err != nil {
					logMessage := fmt.Sprintf("%s 节点 %s 添加失败: %v | %s", ip, name, err, ua)
					log.Printf(logMessage)
					http.Error(w, fmt.Sprintf("添加节点失败: %v", err), http.StatusInternalServerError)
					return
				}
			}

			logMessage := fmt.Sprintf("%s 节点添加成功，名称:%s，Token:%s，地区:%s，城市:%s | %s", ip, name, token, region, city, ua)
			log.Printf(logMessage)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("添加成功"))
			return

		case "Delete":
			name := requestData["Name"].(string)
			id, err := GetIDByName(name)
			if err != nil {
				logMessage := fmt.Sprintf("%s 获取节点ID失败: %v | %s", ip, err, ua)
				log.Printf(logMessage)
				http.Error(w, fmt.Sprintf("获取节点ID失败: %v", err), http.StatusInternalServerError)
				return
			}
			if id > 0 {
				err := DeleteNode(id)
				if err != nil {
					logMessage := fmt.Sprintf("%s 删除节点失败: %v | %s", ip, err, ua)
					log.Printf(logMessage)
					http.Error(w, fmt.Sprintf("删除节点失败: %v", err), http.StatusInternalServerError)
					return
				}
				KickClient(id)

				logMessage := fmt.Sprintf("%s 节点 %s 删除成功 | %s", ip, name, ua)
				log.Printf(logMessage)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("删除成功"))
			} else {
				logMessage := fmt.Sprintf("%s 节点 %s 删除失败，未找到节点 | %s", ip, name, ua)
				log.Printf(logMessage)
				http.Error(w, "未找到节点", http.StatusNotFound)
			}

		case "Update":
			name := requestData["OriginName"].(string)
			NewName := requestData["Name"].(string)
			region := requestData["Region"].(string)
			city := requestData["City"].(string)

			id, err := GetIDByName(name)
			if err != nil {
				logMessage := fmt.Sprintf("%s 获取节点ID失败: %v | %s", ip, err, ua)
				log.Printf(logMessage)
				http.Error(w, fmt.Sprintf("获取节点ID失败: %v", err), http.StatusInternalServerError)
				return
			}

			if id > 0 {
				updateFields := make(map[string]interface{})
				if NewName != "" {
					updateFields["Name"] = NewName
				}
				if region != "" {
					updateFields["Region"] = region
				}
				if city != "" {
					updateFields["City"] = city
				}

				if len(updateFields) == 0 {
					logMessage := fmt.Sprintf("%s 节点 %s 没有需要更新的部分 | %s", ip, name, ua)
					log.Printf(logMessage)
					http.Error(w, "没有需要更新的字段", http.StatusBadRequest)
					return
				}

				err := UpdateNode(id, updateFields)
				if err != nil {
					logMessage := fmt.Sprintf("%s 节点 %s 更新失败: %v | %s", ip, name, err, ua)
					log.Printf(logMessage)
					http.Error(w, fmt.Sprintf("更新节点失败: %v", err), http.StatusInternalServerError)
					return
				}

				logMessage := fmt.Sprintf("%s 节点 %s 更新成功 | %s", ip, name, ua)
				log.Printf(logMessage)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("更新成功"))
			} else {
				logMessage := fmt.Sprintf("%s 节点 %s 未找到 | %s", ip, name, ua)
				log.Printf(logMessage)
				http.Error(w, "未找到节点", http.StatusNotFound)
			}
		default:
			logMessage := fmt.Sprintf("%s 操作无效 | %s", ip, ua)
			log.Printf(logMessage)
			http.Error(w, "无效的操作", http.StatusBadRequest)
		}
	}
}

// DeleteNode 删除节点
func DeleteNode(id int) error {
	err := SQLWrite(`DELETE FROM Node WHERE ID = ?`, id)
	if err != nil {
		return fmt.Errorf("删除节点失败: %w", err)
	}
	return nil
}

func GetIDByName(name string) (int, error) {
	var id int
	querySQL := "SELECT ID FROM Node WHERE Name = ?"
	err := db.QueryRow(querySQL, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("获取节点ID失败: %w", err)
	}
	return id, nil
}
func ClientInfo(r *http.Request) (string, string, string, string, string) {
	var clientAddr, clientKey, clientUA, clientIPType, clientEncoding string
	clientKey = strconv.Itoa(int(crc32.ChecksumIEEE([]byte(r.Header.Get("Sec-Websocket-Key")))))

	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		clientAddr = xForwardedFor
		clientIPType = "X-Forwarded-For"
	} else {
		xRealIP := r.Header.Get("X-Real-IP")
		if xRealIP != "" {
			clientAddr = xRealIP
			clientIPType = "X-Real-IP"
		} else {
			clientAddr = r.RemoteAddr
			host, _, err := net.SplitHostPort(clientAddr)
			if err != nil {
				log.Printf("解析真实地址失败: %v\n", err)
				clientAddr = ""
				clientIPType = ""
			}
			clientAddr = host
			clientIPType = "RemoteIP"
		}
	}

	headers := make(map[string][]string)
	for key, value := range r.Header {
		headers[strings.ToLower(key)] = value
	}

	if r.Header.Get("Accept-Encoding") == "gzip" {
		clientEncoding = "gzip"
	}

	clientUA = r.Header.Get("User-Agent")

	return clientKey, clientAddr, clientIPType, clientUA, clientEncoding
}

// 初始化路由
func initRoutes() {
	http.HandleFunc(config.BroadURI, BroadWS)
	http.HandleFunc(config.NodeURI, NodeWS)
	http.HandleFunc(config.ConsoleURI, Console)
}
