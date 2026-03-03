package tsquery

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

const dialTimeout = 10 * time.Second
const readTimeout = 5 * time.Second

// Client ServerQuery TCP 文本协议最小客户端
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Dial 连接到 ServerQuery
func Dial(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("连接 ServerQuery %s 失败: %w", addr, err)
	}

	c := &Client{conn: conn, reader: bufio.NewReader(conn)}

	// 读取欢迎消息（通常 2-3 行）
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	for i := 0; i < 3; i++ {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(line), "error id=") {
			break
		}
	}
	return c, nil
}

// Close 关闭连接
func (c *Client) Close() {
	_ = c.conn.Close()
}

// Login 登录 ServerQuery
func (c *Client) Login(user, password string) error {
	_, err := c.exec(fmt.Sprintf("login %s %s", user, escapeArg(password)))
	return err
}

// Use 选择虚拟服务器
func (c *Client) Use(sid int) error {
	_, err := c.exec(fmt.Sprintf("use sid=%d", sid))
	return err
}

// ServerList 获取虚拟服务器列表，返回第一个的 virtualserver_id
func (c *Client) ServerList() (int, error) {
	resp, err := c.exec("serverlist")
	if err != nil {
		return 0, err
	}
	// 解析 virtualserver_id=<n>
	for _, part := range strings.Fields(resp) {
		if strings.HasPrefix(part, "virtualserver_id=") {
			var sid int
			if _, err := fmt.Sscanf(part, "virtualserver_id=%d", &sid); err == nil {
				return sid, nil
			}
		}
	}
	return 0, fmt.Errorf("未找到 virtualserver_id")
}

// SetMaxClients 设置最大客户端数（slots）
func (c *Client) SetMaxClients(slots int) error {
	_, err := c.exec(fmt.Sprintf("serveredit virtualserver_maxclients=%d", slots))
	return err
}

// exec 发送命令，读取响应直到出现 error id= 行
func (c *Client) exec(cmd string) (string, error) {
	_ = c.conn.SetDeadline(time.Now().Add(readTimeout))
	if _, err := fmt.Fprintf(c.conn, "%s\n", cmd); err != nil {
		return "", fmt.Errorf("发送命令失败: %w", err)
	}

	var lines []string
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return strings.Join(lines, "\n"), fmt.Errorf("读取响应失败: %w", err)
		}
		trimmed := strings.TrimSpace(line)
		lines = append(lines, trimmed)
		if strings.HasPrefix(trimmed, "error id=") {
			if !strings.Contains(trimmed, "error id=0") {
				return strings.Join(lines, "\n"), fmt.Errorf("ServerQuery 错误: %s", trimmed)
			}
			break
		}
	}
	return strings.Join(lines, "\n"), nil
}

// escapeArg 转义 TS3 参数中的特殊字符
func escapeArg(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, " ", `\s`)
	s = strings.ReplaceAll(s, "|", `\p`)
	return s
}
