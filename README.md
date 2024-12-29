# LightMonitor v0.1
一个轻量的节点监测工具

Demo https://mon.2025202.xyz

## 一键安装
前端项目地址 https://github.com/Lightshadow86/LightMonitor_FE
```
wget https://raw.githubusercontent.com/Lightshadow86/LightMonitor/refs/heads/main/LightMonitor.sh && chmod +x LightMonitor.sh && ./LightMonitor.sh
```
注意：服务端的Token是在前端新增|更新|删除所用的管理密钥，客户端|节点的Token是验证|登录所用。

注：Windows系统的客户端|服务端需要自行编写启动脚本，可以使用以下参考（基于系统任务计划程序）。：
```
schtasks /create /tn "LightMonitor" /tr "\"D:/Program/LightMonitor\" -url wss://api.2025202.xyz/Monitor/Node -token 123456 /sc onstart /rl highest /ru "SYSTEM" /f
```
可实现开机自启（无需登录系统|进入桌面）和隐藏窗口，但可能会导致程序获取到的数据存在某些问题。
如果不是很需要在不登录的情况下进行监控，可以采用开始菜单|启动的方式来运行（需要自行编写vbs来实现隐藏窗口）

## 支持监控项：
CPU、内存、硬盘、网络、进程、负载

## 前身|主要参考|新功能
Akile Monitor https://github.com/akile-network/akile_monitor

在其思路的基础上进行了彻彻底底的重写，使用了更稳妥的连接方式和逻辑，并采用了更合适的数据格式
确保了固定信息仅传输一次，除非发生变化。更节约流量（……好像不差这点）
拥有自动重连（……），中文日志（……），清晰注释（……）的特点……（编不下去了）


但请注意，此版本不支持WebHook以及tg机器人（自然也不兼容AKileMonitorBot，但你可以自己写一个）

## 展望（以后准备做的事|疯狂挖坑）
完善的通知系统：Telegram机器人，钉钉机器人，E-Mail（基于SMTP等），WebHook
更多监控项：电池监控、电源计划（Windows专属），显卡监控、温度监控、风扇监控（可能会使用三方库且大多数可能仍然是Windows专属）
数据记录及统计图展示：后端直接存入数据库，但前端不知道如何做

## 写在最后
由于我本人是个笨蛋，可能也懒了点（……）
这个“项目”加上前端总耗时一个星期，实际用时大约30小时，因为在此之前我从未接触过Go语言，但这一周来的确让我看到了他的魅力
尽管代码是全程由AI写的，但我已尽可能梳理清楚了逻辑。可能在很多地方仍然有不成熟甚至是漏洞的地方，如果你有好的建议，欢迎提issue甚至是直接fork另起分支。

