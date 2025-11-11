package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/RunzhiZhao/long-gate/internal/config"
	"github.com/RunzhiZhao/long-gate/internal/router"
)

func main() {
	// 1. 加载配置并开始监听文件变化
	if err := config.LoadAndWatchConfig("configs/gateway.yaml"); err != nil {
		log.Fatalf("Fatal error loading config: %v", err)
	}

	// 2. 启动路由管理器
	router.InitRouter()

	// 3. 注册网关的入口 Handler
	http.HandleFunc("/", router.HandleRequest)

	// 4. 启动 HTTP 服务器
	port := config.GetGatewayConfig().Port
	log.Printf("long-gate listening on :%d", port)

	addr := fmt.Sprintf(":%d", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
