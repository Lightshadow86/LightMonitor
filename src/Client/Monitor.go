package main

import (
	"fmt"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var lastHost Host

// Host 结构体，存储主机基本信息
type Host struct {
	Platform        string
	PlatformVersion string
	Arch            string
	Virtualization  string
	BootTime        uint64
	CPU             []string
	MemTotal        uint64
	SwapTotal       uint64
	DiskTotal       uint64
}

// HostState 结构体，存储主机实时状态信息
type HostState struct {
	CPU             float64
	Load1           float64
	Load5           float64
	Load15          float64
	MemUsed         uint64
	SwapUsed        uint64
	NetInSpeed      uint64
	NetOutSpeed     uint64
	NetInTransfer   uint64
	NetOutTransfer  uint64
	PacketsRecv     uint64
	PacketsSent     uint64
	PacketsRecvRate float64
	PacketsSentRate float64
	DiskUsed        uint64
	Processes       int
	TCPConections   int
	UDPConnections  int
}

// HostStateInfo 包含主机信息和状态信息
type HostStateInfo struct {
	Host  Host
	State HostState
}

var (
	NetInTransfer, NetOutTransfer    uint64
	lastPacketsRecv, lastPacketsSent uint64
	NetUpdateTimeStamp               uint64
)

// GetHostStateInfo 获取主机信息和实时状态
func GetHostStateInfo() (*HostStateInfo, error) {
	var ret HostStateInfo

	// 获取主机信息
	hi, err := host.Info() // 获取主机信息
	if err != nil {
		return nil, fmt.Errorf("获取主机信息失败: %v", err)
	}

	// 获取操作系统信息
	ret.Host.Platform = hi.Platform
	ret.Host.PlatformVersion = hi.PlatformVersion
	ret.Host.Arch = hi.KernelArch
	ret.Host.BootTime = hi.BootTime
	ret.State.Processes = int(hi.Procs)

	// 获取CPU信息
	ci, err := cpu.Info()
	if err != nil {
		//return nil, fmt.Errorf("获取CPU信息失败: %v", err)
	}
	ret.Host.CPU = append(ret.Host.CPU, strings.ReplaceAll(strings.TrimSpace(fmt.Sprintf(ci[0].ModelName)), "  ", " "))

	// 获取虚拟内存信息
	vm, err := mem.VirtualMemory()
	//if err != nil {
	//	return nil, fmt.Errorf("获取虚拟内存信息失败: %v", err)
	//}
	ret.Host.MemTotal = vm.Total

	// 获取交换空间信息
	swap, err := mem.SwapMemory()
	//if err != nil {
	//	return nil, fmt.Errorf("获取交换空间信息失败: %v", err)
	//}
	ret.Host.SwapTotal = swap.Total

	// 获取主机状态信息
	cp, err := cpu.Percent(0, false) // 获取CPU使用率
	if err != nil || len(cp) == 0 {
		//return nil, fmt.Errorf("获取CPU使用率失败: %v", err)
	}
	ret.State.CPU = Decimal(cp[0]) // 将CPU使用率赋值给ret.State.CPU

	loadStat, err := load.Avg() // 获取系统负载平均值
	if err != nil {
		//return nil, fmt.Errorf("获取系统负载平均值失败: %v", err)
	}
	ret.State.Load1 = Decimal(loadStat.Load1)   // 1分钟平均负载
	ret.State.Load5 = Decimal(loadStat.Load5)   // 5分钟平均负载
	ret.State.Load15 = Decimal(loadStat.Load15) // 15分钟平均负载

	vm, err = mem.VirtualMemory() // 获取虚拟内存信息
	if err != nil {
		//return nil, fmt.Errorf("获取虚拟内存信息失败: %v", err)
	}
	ret.State.MemUsed = vm.Total - vm.Available // 计算已使用内存

	swap, err = mem.SwapMemory() // 获取交换空间信息
	if err != nil {
		//return nil, fmt.Errorf("获取交换空间信息失败: %v", err)
	}
	ret.State.SwapUsed = swap.Used // 已使用交换空间

	// 获取网络流量信息
	var maxNetIn, maxNetOut uint64
	var PacketsRecv, PacketsSent uint64
	var tcpConnections, udpConnections int
	nc, err := net.IOCounters(true) // 获取网络IO计数器
	if err == nil {
		for _, v := range nc {
			name := strings.ToLower(v.Name) // 转为小写方便比较
			if strings.Contains(name, "docker") ||
				strings.Contains(name, "zerotier") ||
				strings.Contains(name, "zt") ||
				strings.Contains(name, "lo") ||
				strings.Contains(name, "br") ||
				strings.Contains(name, "vm") ||
				strings.Contains(name, "wg") ||
				strings.Contains(name, "warp") ||
				strings.Contains(name, "tun") {
				continue // 忽略网卡
			}

			if v.BytesRecv > maxNetIn {
				maxNetIn = v.BytesRecv // 更新最大接收流量
				PacketsRecv = v.PacketsRecv
			}
			if v.BytesSent > maxNetOut {
				maxNetOut = v.BytesSent // 更新最大发送流量
				PacketsSent = v.PacketsSent
			}
		}

		now := uint64(time.Now().Unix()) // 获取当前时间戳
		diff := now - NetUpdateTimeStamp // 计算时间差
		if diff > 0 {
			// 计算接收包的速率（包/秒）
			ret.State.PacketsRecvRate = float64(PacketsRecv-lastPacketsRecv) / float64(diff)
			// 计算发送包的速率（包/秒）
			ret.State.PacketsSentRate = float64(PacketsSent-lastPacketsSent) / float64(diff)

			// 计算网络接收速度（字节/秒）
			ret.State.NetInSpeed = (maxNetIn - NetInTransfer) / diff
			// 计算网络发送速度（字节/秒）
			ret.State.NetOutSpeed = (maxNetOut - NetOutTransfer) / diff

			// 更新接收到的字节数和包数
			ret.State.NetInTransfer = maxNetIn
			ret.State.NetOutTransfer = maxNetOut
			ret.State.PacketsRecv = PacketsRecv // 更新接收的包数
			ret.State.PacketsSent = PacketsSent // 更新发送的包数
		}

		// 记录上次的包数和时间戳
		lastPacketsRecv = PacketsRecv
		lastPacketsSent = PacketsSent
		NetInTransfer = maxNetIn
		NetOutTransfer = maxNetOut
		NetUpdateTimeStamp = now // 更新上次更新时间戳
	}

	// 获取硬盘信息
	disks, err := disk.Partitions(true) // 获取所有分区

	// 正则表达式，过滤掉虚拟文件系统挂载点（如 Docker 等）
	ignorePattern := regexp.MustCompile(`/var/lib/docker|overlay|tmpfs|none|^/dev/loop|^/sys|^/proc|^/run`)

	for _, d := range disks {
		// 如果挂载点匹配正则表达式，跳过该挂载点
		if ignorePattern.MatchString(d.Mountpoint) {
			continue
		}

		// 获取磁盘使用情况
		usage, _ := disk.Usage(d.Mountpoint)
		ret.Host.DiskTotal += usage.Total
		ret.State.DiskUsed += usage.Used
	}

	// 获取网络连接数
	connections, err := net.Connections("all")

	for _, conn := range connections {
		if conn.Laddr.String() == "0.0.0.0:0" || conn.Raddr.String() == "0.0.0.0:0" {
			continue
		}

		isTCP := conn.Family == 2 && conn.Type == 1                        // AF_INET and SOCK_STREAM
		isUDP := (conn.Family == 2 || conn.Family == 10) && conn.Type == 2 // AF_INET or AF_INET6 and SOCK_DGRAM

		if isTCP {
			tcpConnections++
		} else if isUDP {
			udpConnections++
		}
	}
	ret.State.TCPConections = tcpConnections
	ret.State.UDPConnections = udpConnections

	// 返回主机信息和状态信息的结构体
	return &ret, nil
}

// Decimal 保留两位小数
func Decimal(value float64) float64 {
	value, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return value
}

// 比较 Host 信息是否一致
func compareHosts(h1, h2 Host) bool {
	return reflect.DeepEqual(h1, h2)
}
