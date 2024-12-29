package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

var (
	isBroad   bool       = false // 决定是否运行 FetchData 内部逻辑
	BroadData string             // 准备发送的数据
	mutex     sync.Mutex         // 保证多协程下的安全操作
)

// FetchData 整理 Node 表数据
func FetchData() {
	for {
		time.Sleep(1 * time.Second) // 每秒运行一次

		// 如果不需要广播，跳过处理逻辑
		mutex.Lock()
		if !isBroad {
			mutex.Unlock()
			continue
		}
		mutex.Unlock()

		// 读取 Node 表数据
		rows, err := SQLRead("SELECT Data, Status, TimeStamp, Name, Region, City FROM Node")
		if err != nil {
			log.Printf("查询 Node 表失败: %v\n", err)
		}
		if err != nil {
		}

		var servers []map[string]interface{}
		for rows.Next() {
			var hostData, stateData string
			var timestamp int64
			var name, region, city *string

			if err := rows.Scan(&hostData, &stateData, &timestamp, &name, &region, &city); err != nil {
				//log.Printf("读取行数据失败: %v\n", err)
				continue
			}

			var host map[string]interface{}
			var state map[string]interface{}
			if err := json.Unmarshal([]byte(hostData), &host); err != nil {
				//log.Printf("解析 Host 数据失败: %v\n", err)
			}
			if err := json.Unmarshal([]byte(stateData), &state); err != nil {
				//log.Printf("解析 State 数据失败: %v\n", err)
			}

			host["Name"] = name
			host["Region"] = region
			host["City"] = city

			server := map[string]interface{}{
				"Host":      host,
				"State":     state,
				"TimeStamp": timestamp,
			}
			servers = append(servers, server)
		}
		rows.Close()
		dbMutex.RUnlock()

		finalData := map[string]interface{}{
			"Servers":   servers,
			"Timestamp": time.Now().Unix(),
		}
		dataJSON, err := json.Marshal(finalData)
		if err != nil {
			//log.Printf("生成 JSON 数据失败: %v\n", err)
			continue
		}

		// 更新全局变量 BroadData
		mutex.Lock()
		BroadData = string(dataJSON)
		mutex.Unlock()
	}
}
