package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

var ES string

func main() {
	var _ip = flag.String("ip", "", "指定要连接的IP或者域名")
	var _pt = flag.String("port", "80", "指定要尝试连接的端口")
	var _tp = flag.String("t", "tcp", "端口类型 [TCP|UDP]")
	var _es = flag.String("es", "", "将数据导入es，指定路径如 http://127.0.0.1:9200/index/type/")
	var _he = flag.String("help", "", "显示帮助信息")

	flag.Parse()

	if len(os.Args) == 2 {
		*_ip = os.Args[1]
	}

	ES = *_es

	if len(*_ip) == 0 || len(*_he) > 0 {
		fmt.Println("一个简单的网络连接对比测试工具")
		flag.PrintDefaults()
		os.Exit(0)
	}

	for {
		go MonitorService(*_ip, *_pt, *_tp, 0, 10)
		time.Sleep(1 * time.Second)
	}
}

func MonitorService(ip string, port string, tcporudp string, timeout int, retry int) (latency float64, status string, status_new int) {
	latency = 0
	status = "OK" //[]string{"ok", "timeout", "miss partern"}
	result := true

	s := time.Now()

	conn, err := net.DialTimeout(tcporudp, fmt.Sprintf("%s:%s", ip, port), time.Duration(timeout)*time.Second)
	if err != nil {
		status = err.Error()
		result = false
	} else {
		defer conn.Close()
	}

	e := time.Now()
	latency = float64(e.UnixNano()-s.UnixNano()) / 1000000000

	log := fmt.Sprintf(`{"date":"%s", "ip":"%s", "port":%s, "type":"%s", "status":"%s", "latency":%f}`,
		time.Now().Format("2006-01-02T15:04:05+08:00"), ip, port, tcporudp, status, latency)

	if ES != "" {
		go exec.Command("/usr/bin/curl", "-XPOST", ES, "--data-binary", log).Start()
	}

	fmt.Println(log)

	if !result && retry > 1 {
		time.Sleep(1 * time.Second)
		return MonitorService(ip, port, tcporudp, timeout, retry-1)
	}

	status_new = 1
	if !result {
		status_new = 0
	}

	return latency, status, status_new
}
