package tsquery_test

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"
	"ts-panel/src/tsquery"
)

// mockTSServer 启动一个模拟 ServerQuery 服务器
func mockTSServer(t *testing.T, handler func(conn net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动 mock 服务器失败: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}()
	return ln.Addr().String()
}

func TestDial_Success(t *testing.T) {
	addr := mockTSServer(t, func(conn net.Conn) {
		// 发送欢迎信息
		_, _ = fmt.Fprintf(conn, "TS3\n")
		_, _ = fmt.Fprintf(conn, "Welcome to the TeamSpeak 3 ServerQuery interface\n")
	})

	time.Sleep(50 * time.Millisecond)
	c, err := tsquery.Dial(addr)
	if err != nil {
		t.Fatalf("Dial 失败: %v", err)
	}
	defer c.Close()
}

func TestLogin_Success(t *testing.T) {
	addr := mockTSServer(t, func(conn net.Conn) {
		// 欢迎消息
		_, _ = fmt.Fprintf(conn, "TS3\n")
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) > 0 {
				// 返回成功响应
				_, _ = fmt.Fprintf(conn, "error id=0 msg=ok\n")
			}
		}
	})

	time.Sleep(50 * time.Millisecond)
	c, err := tsquery.Dial(addr)
	if err != nil {
		t.Fatalf("Dial 失败: %v", err)
	}
	defer c.Close()

	if err := c.Login("serveradmin", "testpass"); err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
}

func TestSetMaxClients_ErrorResponse(t *testing.T) {
	addr := mockTSServer(t, func(conn net.Conn) {
		_, _ = fmt.Fprintf(conn, "TS3\n")
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			// 对任何命令返回错误
			_, _ = fmt.Fprintf(conn, "error id=512 msg=not\\sconnected\n")
		}
	})

	time.Sleep(50 * time.Millisecond)
	c, err := tsquery.Dial(addr)
	if err != nil {
		t.Fatalf("Dial 失败: %v", err)
	}
	defer c.Close()

	err = c.SetMaxClients(10)
	if err == nil {
		t.Error("期望返回错误，但未返回")
	}
}
