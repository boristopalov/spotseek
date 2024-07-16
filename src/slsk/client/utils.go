package client

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

func (c *SlskClient) String() string {
	json, err := c.Json()
	if err != nil {
		return ""
	}
	return string(json)
}

func (c *SlskClient) Json() ([]byte, error) {
	json, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return nil, err
	}
	return json, nil
}

func SplitHostPort(conn net.Conn) (string, uint32, error) {
	ip, portStr, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("error parsing port: %w", err)
	}
	return ip, uint32(port), nil
}
