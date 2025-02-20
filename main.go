package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	Version = "0.0.6"
	BaseApi = ""
	Token   = ""
	Region  = ""
	Support = []string{"website", "service"}
	Header  = map[string]string{}
	wg      sync.WaitGroup
)

type JsonStr map[string]interface{}

type ApiErr struct {
	ErrMsg string `json:"error"`
}

type ApiList struct {
	ServerId string `json:"server_id"`
	Label    string `json:"label"`
	Type     string `json:"type"`
}

type ApiItem struct {
	ErrMsg   string `json:"error"`
	ServerId string `json:"server_id"`
	Uri      string `json:"ip"`
	Port     string `json:"port"`
	Type     string `json:"type"`
	Partern  string `json:"pattern"`
	Timeout  string `json:"timeout"`
	Username string `json:"website_username"`
	Password string `json:"website_password"`
	Maxretry string `json:"max_retry"`
	HeadName string `json:"header_name"`
	HeadVal  string `json:"header_value"`
	Headers  string `json:"headers"`
}

func main() {
	hostname, _ := os.Hostname()

	var _url = flag.String("url", "", "psm server url")
	var _token = flag.String("token", "", "psm api token")
	var _region = flag.String("region", hostname, "region or node ID, use hostname default")
	var _help = flag.String("help", "", "show help message")

	flag.Parse()

	BaseApi = *_url
	Token = *_token
	Region = *_region

	if len(BaseApi) == 0 || len(*_help) > 0 {
		fmt.Println("phpservermon Client for go.\nhttps://github.com/vus520/psm_go_client\nversion " + Version)
		flag.PrintDefaults()
		os.Exit(0)
	}

	Header["User-Agent"] = "Mozilla/5.0 (phpservermon go; " + Region + "; " + Version + ") AppleWebKit/537.36 (KHTML, like Gecko)"

	MoniorStart()

	wg.Wait()
}

func MoniorStart() {
	url := fmt.Sprintf("%s/?mod=api&token=%s", BaseApi, token())

	fmt.Println("Fetch apilist from " + url)

	data, err := curl(url, 60, Header)
	if err != nil {
		fmt.Println("can't format json result: " + err.Error())
		fmt.Println(url)
		fmt.Println(data)

		return
	}

	List := []ApiList{}

	//请求异常
	keys := getKeys(data)
	if inSlice("error", keys) {
		fmt.Println("some error:" + data)
		return
	}

	if err := json.Unmarshal([]byte(data), &List); err != nil {
		fmt.Println("can't format json result: " + err.Error())
		fmt.Println(url)
		fmt.Println(data)
		return
	} else {

		fmt.Printf("Found %d items\n", len(List))

		for _, v := range List {
			//处理支持的检测类型
			if inSlice(v.Type, Support) {
				wg.Add(1)
				go MoniorItem(v.ServerId)
			}

		}
	}
}

func MoniorItem(ServerId string) {
	defer wg.Done()

	url := fmt.Sprintf("%s/?mod=api&action=server&server_id=%s&token=%s", BaseApi, ServerId, token())

	data, err := curl(url, 60, Header)

	if err != nil {
		panic(err.Error())
	}

	//请求异常
	keys := getKeys(data)
	if len(keys) == 0 || inSlice("error", keys) {
		fmt.Println("some error:" + data)
		return
	}

	Item := ApiItem{}

	if err := json.Unmarshal([]byte(data), &Item); err != nil {
		fmt.Println("can't format json result: " + err.Error())
		fmt.Println(url)
		fmt.Println(data)
		return
	} else {
		//把公用参数提出来统一处理
		timeout, err := strconv.Atoi(Item.Timeout)
		if err != nil || timeout < 1 || timeout > 100 {
			timeout = 20
		}

		//检查超时时，可以进行重试
		Maxretry, err := strconv.Atoi(Item.Maxretry)
		if err != nil || Maxretry > 10 || Maxretry < 2 {
			Maxretry = 2
		}

		var (
			latency    float64
			status_msg string
			status_new int
		)

		switch Item.Type {
		case "website":
			latency, status_msg, status_new = MonitorWebsite(Item, timeout, Maxretry)
		case "service":
			latency, status_msg, status_new = MonitorService(Item, timeout, Maxretry)
		}

		api := fmt.Sprintf("%s/?mod=api&action=update&server_id=%s&status=%v&error=%s&latency=%f&region=%s&token=%s",
			BaseApi, ServerId, status_new, status_msg, latency, Region, token())

		_, err = curl(api, 10, Header)
		if err != nil {
			fmt.Println("api callback error:", api, err.Error())
		}
	}
}

func MonitorWebsite(item ApiItem, timeout int, retry int) (latency float64, status string, status_new int) {
	latency = 0
	status = "OK" //[]string{"ok", "timeout", "miss partern"}
	result := true
	partern := item.Partern

	s := time.Now()

	//URL监控有时候需要设置头信息，则复制全局头信息，并添加监控项的头信息
	header := map[string]string{}
	for k, v := range Header {
		header[k] = v
	}
	if len(item.Headers) > 0 {
		s := strings.Split(item.Headers, ";")
		for _, v := range s {
			ss := strings.Split(v, ":")

			if len(ss) < 2 {
				continue
			}
			header[strings.TrimSpace(ss[0])] = strings.TrimSpace(ss[1])
		}
	}

	data, err := curl(item.Uri, time.Duration(timeout), header)
	if err != nil {
		status = err.Error()
		result = false
	} else {
		//成功获取目标页面，检查是否有匹配内容
		if len(partern) > 0 && !Contains(data, partern) {
			status = "MissPartern"
			result = false
		}

		if data == "" && partern == ".+" {
			status = "OK"
			result = true
		}
	}

	e := time.Now()
	latency = float64(e.UnixNano()-s.UnixNano()) / 1000000000

	//@todo if latency > float64(timeout) , status = "Timeout"

	fmt.Printf("URI: %s, data:%d, Status: %s, Latency:%f\n", item.Uri, len(data), status, latency)

	if !result && retry > 1 {
		time.Sleep(1 * time.Second)
		return MonitorWebsite(item, timeout, retry-1)
	}

	status_new = 1
	if !result {
		status_new = 0
	}

	return latency, status, status_new
}

func MonitorService(item ApiItem, timeout int, retry int) (latency float64, status string, status_new int) {
	latency = 0
	status = "OK" //[]string{"ok", "timeout", "miss partern"}
	result := true

	s := time.Now()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", item.Uri, item.Port), time.Duration(timeout)*time.Second)
	if err != nil {
		status = err.Error()
		result = false
	} else {
		defer conn.Close()
	}

	e := time.Now()
	latency = float64(e.UnixNano()-s.UnixNano()) / 1000000000

	fmt.Printf("IP: %s, port:%s, Status: %s, Latency:%f\n", item.Uri, item.Port, status, latency)

	if !result && retry > 1 {
		time.Sleep(1 * time.Second)
		return MonitorService(item, timeout, retry-1)
	}

	status_new = 1
	if !result {
		status_new = 0
	}

	return latency, status, status_new
}

func Contains(str string, partern string) bool {
	if len(str) == 0 {
		return false
	}

	//如果找不到字符串就通过正则获取, 最好检测是否是正则表达式
	if strings.Contains(str, partern) {
		return true
	}

	matched, err := regexp.MatchString(partern, str)

	if !matched || err != nil {
		return false
	}

	return true
}

func getKeys(data string) []string {
	Item := JsonStr{}
	Keys := []string{}

	if err := json.Unmarshal([]byte(data), &Item); err != nil {
		return Keys
	}

	for k := range Item {
		Keys = append(Keys, k)
	}

	return Keys
}

func inSlice(val string, slice []string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func curl(url string, TimeOut time.Duration, header map[string]string) (string, error) {
	timeout := time.Duration(TimeOut * time.Second)

	client := http.Client{
		Timeout:       timeout,
		CheckRedirect: redirectPolicy,
	}

	req, err := http.NewRequest("GET", url, nil)

	//Pay attention that in http.Request header "Host" can not be set via Set method
	//but can be set directly:
	//req.Host = "domain.tld":
	for k, v := range header {
		if k == "Host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", errors.New(resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	return string(body), nil
}

func redirectPolicy(req *http.Request, via []*http.Request) error {
	//@todo 检查是否有跨域的跳转
	//fmt.Println(req.URL.String())

	//所有301,302都进行跳转
	return nil
}

func token() string {
	t := fmt.Sprintf("%d", time.Now().Unix())

	s := fmt.Sprintf("%s%s", t, Token)
	s = fmt.Sprintf("%x", md5.Sum([]byte(t)))

	e := fmt.Sprintf("%s%s", t, s[0:len(t)])

	return e
}
