package net

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config 请求包初始化参数配置
type Config struct {
	BaseURL  *url.URL      // 请求根地址
	Overtime time.Duration // 网络超时时间
}

// Net 请求结构体
type Net struct {
	client   *http.Client  // 可重复使用的client
	baseURL  *url.URL      // 请求根地址
	overtime time.Duration // 网络超时时间
}

// SuperAgent 请求参数
type SuperAgent struct {
	net         *Net        // 当前请求包实例
	url         string      // 请求地址
	method      string      // 请求方式
	contentType string      // 请求类型
	body        interface{} // 发送请求的body
}

const (
	contentJSON = "application/json;charset=utf-8"
	contentXML  = "application/xml;charset=utf-8"
	contentText = "text/plain;charset=utf-8"
)

// New 初始化一个请求包对象
func New(c *Config) *Net {
	if c == nil {
		c = &Config{Overtime: time.Second * 10}
	}
	return &Net{
		client: http.DefaultClient, baseURL: c.BaseURL, overtime: c.Overtime,
	}
}

// Get 发送 Get 请求
func (n *Net) Get(url string) *SuperAgent {
	return &SuperAgent{net: n, url: url, method: "GET"}
}

// Post 发送 Post 请求
func (n *Net) Post(url string) *SuperAgent {
	return &SuperAgent{net: n, url: url, method: "POST"}
}

// Put 发送 Put 请求
func (n *Net) Put(url string) *SuperAgent {
	return &SuperAgent{net: n, url: url, method: "PUT"}
}

// Delete 发送 Delete 请求
func (n *Net) Delete(url string) *SuperAgent {
	return &SuperAgent{net: n, url: url, method: "DELETE"}
}

// JSON 设置请求数据内容，默认用 Content-Type=application/json; 方式发送json数据
func (s *SuperAgent) JSON(body interface{}) *SuperAgent {
	s.body = body
	s.contentType = contentJSON
	return s
}

// XML 设置请求数据内容，默认用 Content-Type=application/json; 方式发送json数据
func (s *SuperAgent) XML(body interface{}) *SuperAgent {
	s.body = body
	s.contentType = contentXML
	return s
}

// Text 设置请求数据内容，默认用 Content-Type=text/plain; 方式发送string数据
func (s *SuperAgent) Text(body string) *SuperAgent {
	s.body = body
	s.contentType = contentText
	return s
}

// End 开始http请求
func (s *SuperAgent) End(ctx context.Context, v interface{}) (*http.Response, error) {
	if len(s.contentType) > 0 && s.body == nil {
		s.body = ""
	}
	var req *http.Request
	var err error
	buf := new(bytes.Buffer)
	switch s.contentType {
	case contentJSON:
		err = json.NewEncoder(buf).Encode(s.body)
	case contentXML:
		err = xml.NewEncoder(buf).Encode(s.body)
	case contentText:
	default:
		buf = nil
	}
	if err != nil {
		return nil, err
	}

	// 转换 url
	rel, err := url.Parse(s.url)
	if err != nil {
		return nil, err
	}
	u := s.net.baseURL.ResolveReference(rel)

	req, err = http.NewRequest(s.method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if len(s.contentType) > 0 {
		req.Header.Set("Content-Type", contentText)
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	// 执行网络请求
	resp, err := s.net.client.Do(req)
	if err != nil {
		// If we got an error, and the context has been canceled, the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If the error type is *url.Error, sanitize its URL before returning.
		if e, ok := err.(*url.Error); ok {
			if url, err := url.Parse(e.URL); err == nil {
				e.URL = url.String()
				return nil, e
			}
		}
		return nil, err
	}

	defer func() {
		// Drain up to 512 bytes and close the body to let the Transport reuse the connection
		io.CopyN(ioutil.Discard, resp.Body, 512)
		resp.Body.Close()
	}()

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			io.Copy(w, resp.Body)
		} else {
			body, err := ioutil.ReadAll(resp.Body)
			if !strings.Contains(string(body), "ip_list") {
				log.Printf("url %s body %s", req.URL.Path, string(body))
			}

			// 默认认为 contentType 不为xml的情况下，所有返回都用json解析
			if strings.EqualFold(s.contentType, contentXML) {
				err = xml.Unmarshal(body, v)
			} else {
				err = json.Unmarshal(body, v)
			}

			if err == io.EOF {
				err = nil // ignore EOF errors caused by empty response body
			}
		}
	}

	return resp, err
}