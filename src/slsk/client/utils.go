package client

import "encoding/json"

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
