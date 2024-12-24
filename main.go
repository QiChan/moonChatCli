package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn   *websocket.Conn
	done   chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
}

func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *Client) connect(url string) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %v", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		close(c.done)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("read error: %v", err)
				}
				return
			}
			log.Printf("received: %s", message)
		}
	}
}

func (c *Client) writeMessage(message string) error {
	return c.conn.WriteMessage(websocket.TextMessage, []byte(message))
}

func (c *Client) close() {
	c.cancel()
	// 发送关闭消息
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Printf("write close message error: %v", err)
	}
	// 等待读取泵结束
	<-c.done
}

func main() {
	// 创建客户端
	client := NewClient()

	// 连接WebSocket服务器
	err := client.connect("ws://127.0.0.1:8091/ws")
	if err != nil {
		log.Fatal("connect error:", err)
	}
	log.Println("Connected to WebSocket server")

	// 启动读取泵
	go client.readPump()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建一个用于发送消息的 goroutine
	go func() {
		fmt.Println("输入消息按回车发送，按 Ctrl+C 退出:")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			message := scanner.Text()
			if err := client.writeMessage(message); err != nil {
				log.Printf("发送消息错误: %v", err)
			}
		}
	}()

	// 等待中断信号
	<-sigChan
	log.Println("\nReceived shutdown signal")

	// 优雅关闭
	client.close()
	log.Println("Connection closed properly")
}
