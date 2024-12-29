package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	_ "modernc.org/sqlite" // SQLite 驱动
	"strings"
	"sync"
)

// 数据库全局变量
var db *sql.DB
var dbMutex sync.RWMutex // 读写锁

// SQLWrite 数据库写入
func SQLWrite(query string, args ...interface{}) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("数据库写入失败: %w", err)
	}
	return nil
}

// SQLRead 数据库读取
func SQLRead(query string, args ...interface{}) (*sql.Rows, error) {
	// 获取读锁，允许并发读取
	dbMutex.RLock()
	// 执行SQL读取操作
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("数据库读取失败: %w", err)
	}
	return rows, nil
}

// InitDatabase 初始化数据库
func InitDatabase(cfg DatabaseConfig) error {
	var err error
	db, err = sql.Open("sqlite", cfg.FilePath)
	if err != nil {
		return fmt.Errorf("无法连接到数据库: %v", err)
	}

	// 创建必要的表
	err = createNodeTable()
	if err != nil {
		return fmt.Errorf("初始化 Node 表失败: %v", err)
	}
	err = createClientTable()
	if err != nil {
		return fmt.Errorf("初始化 Client 表失败: %v", err)
	}

	// 启动时清空 Client 表
	_, err = db.Exec("DELETE FROM Client")
	if err != nil {
		return fmt.Errorf("清空 Client 表失败: %v", err)
	}

	//log.Println("数据库连接成功")
	return nil
}

// 创建表 Node
func createNodeTable() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS Node (
		ID INTEGER PRIMARY KEY AUTOINCREMENT,
		Name TEXT NOT NULL,
		Token TEXT NOT NULL,
		Region TEXT,
		City TEXT,
		IP TEXT,
		Data TEXT,
		Status TEXT,
		Timestamp INTEGER DEFAULT (strftime('%s', 'now'))
	);
	`
	err := SQLWrite(createTableSQL)
	if err != nil {
		log.Printf("创建数据表失败")
		return nil
	}
	return err
}

// AddNode 添加新节点
func AddNode(name, token, region, city string) error {
	data := struct {
		Arch            string   `json:"Arch"`
		BootTime        int64    `json:"BootTime"`
		CPU             []string `json:"CPU"`
		DiskTotal       int64    `json:"DiskTotal"`
		MemTotal        int64    `json:"MemTotal"`
		Platform        string   `json:"Platform"`
		PlatformVersion string   `json:"PlatformVersion"`
		SwapTotal       int64    `json:"SwapTotal"`
		Virtualization  string   `json:"Virtualization"`
	}{
		CPU: []string{},
	}

	status := struct {
		CPU             float64 `json:"CPU"`
		DiskUsed        int64   `json:"DiskUsed"`
		Load1           float64 `json:"Load1"`
		Load15          float64 `json:"Load15"`
		Load5           float64 `json:"Load5"`
		MemUsed         int64   `json:"MemUsed"`
		NetInSpeed      int64   `json:"NetInSpeed"`
		NetInTransfer   int64   `json:"NetInTransfer"`
		NetOutSpeed     int64   `json:"NetOutSpeed"`
		NetOutTransfer  int64   `json:"NetOutTransfer"`
		PacketsRecv     int64   `json:"PacketsRecv"`
		PacketsRecvRate int64   `json:"PacketsRecvRate"`
		PacketsSent     int64   `json:"PacketsSent"`
		PacketsSentRate int64   `json:"PacketsSentRate"`
		Processes       int64   `json:"Processes"`
		SwapUsed        int64   `json:"SwapUsed"`
		TCPConections   int64   `json:"TCPConections"`
		UDPConnections  int64   `json:"UDPConnections"`
	}{}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化数据字段失败: %w", err)
	}
	statusJSON, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("序列化状态字段失败: %w", err)
	}

	insertSQL := `INSERT INTO Node (Name, Token, Region, City, IP, Data, Status, Timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	err = SQLWrite(insertSQL, name, token, region, city, "", string(dataJSON), string(statusJSON), 0)
	if err != nil {
		return fmt.Errorf("插入数据失败: %w", err)
	}
	return nil
}

// UpdateNode 更新节点信息
func UpdateNode(id int, updates map[string]interface{}) error {
	// 构建动态更新语句
	setClauses := ""
	args := []interface{}{}

	for key, value := range updates {
		setClauses += fmt.Sprintf("%s = ?, ", key)
		args = append(args, value)
	}
	// 移除最后的逗号和空格
	setClauses = setClauses[:len(setClauses)-2]

	// 添加条件参数
	args = append(args, id)

	dbMutex.Lock()
	updateSQL := fmt.Sprintf("UPDATE Node SET %s WHERE ID = ?;", setClauses)
	_, err := db.Exec(updateSQL, args...)
	dbMutex.Unlock()
	if err != nil {
		return fmt.Errorf("更新节点失败")
	}
	return nil
}

// 创建表 Client
func createClientTable() error {
	dbMutex.Lock()
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS Client (
		UID TEXT PRIMARY KEY,
		ID TEXT,
		IP TEXT,
		IPType TEXT,
		Type TEXT,
		UA TEXT,
		TimeStamp INTEGER
	);
	`
	_, err := db.Exec(createTableSQL)
	dbMutex.Unlock()
	return err
}

// WhoAreYou 判断 UA 是否包含 LightMonitor
func WhoAreYou(key string) string {
	if strings.Contains(key, "Node") {
		return "节点"
	}
	return "前端"
}

// GetData 获取节点数据
func GetData(clientID int, data map[string]interface{}) error {
	var host, state string

	// 提取并判断 Host 和 State，并确保它们是可以插入数据库的类型（字符串格式）
	if hostRaw, exists := data["Host"]; exists {
		// 判断 Host 是否为一个 map 类型
		if hostMap, ok := hostRaw.(map[string]interface{}); ok {
			// 将 Host map 转换为 JSON 字符串
			hostBytes, err := json.Marshal(hostMap)
			if err != nil {
				//return fmt.Errorf("Host 字段转换为 JSON 失败: %v", err)
			}
			host = string(hostBytes) // 将转换后的 JSON 字符串赋值给 host
		} else {
			// 如果 Host 不是 map 类型，则设置为空
			host = ""
		}
	}

	if stateRaw, exists := data["State"]; exists {
		// 判断 State 是否为一个 map 类型
		if stateMap, ok := stateRaw.(map[string]interface{}); ok {
			// 将 State map 转换为 JSON 字符串
			stateBytes, err := json.Marshal(stateMap)
			if err != nil {
				//return fmt.Errorf("State 字段转换为 JSON 失败: %v", err)
			}
			state = string(stateBytes) // 将转换后的 JSON 字符串赋值给 state
		} else {
			// 如果 State 不是 map 类型，则设置为空
			state = ""
		}
	} else {
		return fmt.Errorf("State 字段缺失")
	}

	// 更新数据库中的 Node 表，更新 Data、State 和 Timestamp
	dbMutex.Lock()
	updateSQL := `UPDATE Node 
              SET Data = CASE 
                           WHEN ? THEN ? 
                           ELSE Data 
                         END, 
                  Status = ?, 
                  Timestamp = strftime('%s', 'now') 
              WHERE ID = ?`
	_, err := db.Exec(updateSQL, host != "", host, state, clientID)
	dbMutex.Unlock()
	if err != nil {
		//return fmt.Errorf("更新数据库失败: %w", err)
	}

	return nil
}
