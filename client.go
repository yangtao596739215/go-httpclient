package gohttpclient

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/yangtao596739215/go-httpclient/utils"
	"stathat.com/c/consistent"
)

//go get stathat.com/c/consistent@v1.0.0

type AddrProducer func() ([]string, error)

var ErrInitAddr = errors.New("init http addr failed")

const (
	DefaultNumberOfReplicas = 200
	UpdateAddrIntervalMs    = 3000
	ContentTypeProto        = "application/proto"
)

type HttpClient struct {
	client           *http.Client
	consistentClient *consistent.Consistent
	Addrs            []string
	addrProducer     AddrProducer //返回ip:port列表
}

type ClientInitOption struct {
	Addrs            []string
	DialTimeoutMs    int64
	RequestTimeoutMs int64
	MaxIdleConn      int
}

func NewHttpClient() *HttpClient {
	return &HttpClient{}
}

func (c *HttpClient) Init(option *ClientInitOption) error {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(option.DialTimeoutMs) * time.Millisecond, //底层tcp的超时
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          option.MaxIdleConn,
			IdleConnTimeout:       1000 * time.Millisecond,
			TLSHandshakeTimeout:   1000 * time.Millisecond,
			ExpectContinueTimeout: 1000 * time.Millisecond,
		},
		Timeout: time.Duration(option.RequestTimeoutMs) * time.Millisecond, //http的超时
	}

	c.client = client

	c.consistentClient = consistent.New()
	c.consistentClient.NumberOfReplicas = DefaultNumberOfReplicas

	//尝试获取，获取失败则赋默认值
	if c.addrProducer != nil {
		addrList, err := c.addrProducer()
		if err != nil {
			//log warn
			c.Addrs = option.Addrs
		} else {
			fmt.Printf("curr list:%v", addrList)
			c.Addrs = addrList
		}
	} else {
		c.Addrs = option.Addrs
	}

	for _, addr := range c.Addrs {
		c.consistentClient.Add(addr)
	}

	//定时更新
	if c.addrProducer != nil {
		go c.updateAddrList()
	}

	//如果初始化完了,依然没有获取到地址，则报错
	if c.Addrs == nil {
		return ErrInitAddr
	}

	return nil
}

func (c *HttpClient) updateAddrList() {
	var addrList []string
	addrList = append(addrList, c.Addrs...)

	defer func() {
		if err, ok := recover().(error); ok {
			//log stack
			buf := make([]byte, 10000)
			runtime.Stack(buf, false)
			fmt.Printf("panic cover, panic info:%s", string(buf))
			fmt.Println(err)
		}
		go c.updateAddrList()
	}()

	for {
		newAddrList, err := c.addrProducer()
		if err != nil { //获取失败则打印日志，不更新
			//logxxx
		} else { //获取成功则更新
			addList, delList := utils.AddrListDiff(addrList, newAddrList)
			fmt.Printf("[producer:%+vm add addr:%v, del:%v]", c.addrProducer, addList, delList)
			for _, addr := range addList {
				c.consistentClient.Add(addr)
			}
			for _, addr := range delList {
				c.consistentClient.Remove(addr)
			}
			addrList = newAddrList
		}
		time.Sleep(time.Duration(UpdateAddrIntervalMs) * time.Millisecond)
	}
}

//传入做一致hash的key，获取对应的ip:port
func (c *HttpClient) GetHostAddr(key string) (string, error) {
	if c.addrProducer != nil {
		addr, err := c.consistentClient.Get(key)
		if err != nil {
			return "", nil
		}
		return addr, nil
	}
	//如果没传producer，则返回默认的
	return c.Addrs[0], nil
}

func (c *HttpClient) CallMethod(path, key string,
	req []byte) ([]byte, error) {
	reader := bytes.NewReader(req)
	addr, err := c.GetHostAddr(key)
	if err != nil {
		return nil, err
	}

	if addr == "" {
		//log warn
		fmt.Printf("get addr fail, path:%s", path)
		return nil, errors.New("get no valid addr")
	}
	url := utils.MakeUrl(addr, path)
	resp, err := c.client.Post(url, ContentTypeProto, reader)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return res, nil
}
