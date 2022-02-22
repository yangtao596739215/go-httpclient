package main

import (
	gohttpclient "github.com/yangtao596739215/go-httpclient"
)

func main() {
	client := gohttpclient.NewHttpClient()
	option := &gohttpclient.ClientInitOption{
		Addrs:       []string{"127.0.0.1"},
		MaxIdleConn: 20,
	}
	err := client.Init(option)
	if err != nil {
		panic("init http client err")
	}

}
