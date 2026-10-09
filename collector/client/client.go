/*
 Copyright (c) 2024-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"powerstore-metrics-exporter/utils"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type Client struct {
	authMu   sync.RWMutex
	IP       string
	username string
	password string
	version  string
	limit    int
	baseUrl  string
	http     *http.Client
	token    string
	cookie   string
	logger   log.Logger
}

func NewClient(config utils.Storage, logger log.Logger) (*Client, error) {
	var limit int
	if config.Ip == "" || config.User == "" || config.Password == "" || config.Version == "" {
		return nil, errors.New("please check config file ,Some parameters are null")
	}
	if config.Limit <= 0 || config.Limit > 2000 {
		limit = 2000
	} else {
		limit = config.Limit
	}
	baseUrl := "https://" + config.Ip + "/api/rest/"
	tlsConfig, err := utils.ArrayTLSConfig(config)
	if err != nil {
		return nil, err
	}
	var httpClient *http.Client
	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       tlsConfig,
		},
		Timeout: 60 * time.Second,
	}
	client := &Client{
		IP:       config.Ip,
		username: config.User,
		password: config.Password,
		version:  config.Version,
		limit:    limit,
		baseUrl:  baseUrl,
		http:     httpClient,
		logger:   logger,
	}
	return client, client.InitLogin()
}

func (c *Client) InitLogin() error {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	reqUrl := c.baseUrl + "login_session"
	request, err := http.NewRequest("GET", reqUrl, bytes.NewBuffer([]byte("")))
	if err != nil {
		return err
	}
	request.SetBasicAuth(c.username, c.password)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		level.Warn(c.logger).Log("msg", "Request URL error!")
		return err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated:
		c.token = response.Header.Get("Dell-Emc-Token")
		cookies := response.Cookies()
		for _, cookie := range cookies {
			if cookie.Name == "auth_cookie" {
				c.cookie = cookie.Value
			}
		}
		return nil
	default:
		body, err := io.ReadAll(response.Body)
		level.Warn(c.logger).Log("msg", "get token error", "err", err)
		return errors.New("get token error: " + string(body))
	}
}

// getResource follows Content-Range instead of assuming a configured page size
// was honored by the array. A failed page invalidates the entire collection.
func (c *Client) getResource(method, uri, body string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	query := u.Query()
	if method == http.MethodGet {
		limit := c.limit
		if limit <= 0 || limit > 2000 {
			limit = 2000
		}
		query.Set("limit", strconv.Itoa(limit))
		if query.Get("order") == "" {
			query.Set("order", "id")
		}
	}
	offset := 0
	expectedTotal := -1
	var all []json.RawMessage
	for page := 0; page < 10000; page++ {
		if method == http.MethodGet {
			query.Set("offset", strconv.Itoa(offset))
			u.RawQuery = query.Encode()
		}
		data, status, contentRange, err := c.requestPage(method, u.String(), body)
		if err != nil {
			return "", err
		}
		if status != http.StatusPartialContent && page == 0 && contentRange == "" {
			return data, nil
		}
		if method != http.MethodGet {
			return "", fmt.Errorf("partial response for non-paginated request")
		}
		var rows []json.RawMessage
		if err = json.Unmarshal([]byte(data), &rows); err != nil || !strings.HasPrefix(strings.TrimSpace(data), "[") {
			return "", fmt.Errorf("invalid paginated collection")
		}
		if status == http.StatusOK && contentRange == "" && expectedTotal >= 0 && len(rows) == expectedTotal-offset {
			all = append(all, rows...)
			result, err := json.Marshal(all)
			return string(result), err
		}
		var start, end, total int
		// PowerStore documents Content-Range as 0-99/1000 (without a unit).
		n, err := fmt.Sscanf(strings.TrimPrefix(contentRange, "items "), "%d-%d/%d", &start, &end, &total)
		if err != nil || n != 3 || start != offset || end < start || end >= total || len(rows) != end-start+1 {
			return "", fmt.Errorf("invalid pagination range")
		}
		if expectedTotal >= 0 && total != expectedTotal {
			return "", fmt.Errorf("inventory changed during pagination; retry collection")
		}
		expectedTotal = total
		all = append(all, rows...)
		if end+1 == total {
			result, err := json.Marshal(all)
			return string(result), err
		}
		offset = end + 1
	}
	return "", fmt.Errorf("pagination exceeded safety limit")
}

func (c *Client) requestPage(method, uri, body string) (string, int, string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		request, err := http.NewRequest(method, c.baseUrl+uri, strings.NewReader(body))
		if err != nil {
			return "", 0, "", err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")
		c.authMu.RLock()
		request.Header.Set("DELL-EMC-TOKEN", c.token)
		request.Header.Set("Cookie", "auth_cookie="+c.cookie)
		c.authMu.RUnlock()
		request.Header.Set("dell-visibility", "Internal")
		response, err := c.http.Do(request)
		if err != nil {
			return "", 0, "", err
		}
		data, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return "", 0, "", readErr
		}
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusFound {
			if attempt == 0 {
				if err := c.InitLogin(); err != nil {
					return "", 0, "", err
				}
				continue
			}
			return "", 0, "", fmt.Errorf("authentication failed after retry")
		}
		switch response.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusPartialContent:
			return string(data), response.StatusCode, response.Header.Get("Content-Range"), nil
		default:
			return "", 0, "", fmt.Errorf("PowerStore request failed: HTTP %d", response.StatusCode)
		}
	}
	return "", 0, "", fmt.Errorf("authentication failed")
}
