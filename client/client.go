package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"

	"git.mills.io/yarnsocial/yarn"
	"git.mills.io/yarnsocial/yarn/types"
)

const (
	APIVersion = 1
)

var (
	// DefaultUserAgent ...
	DefaultUserAgent = fmt.Sprintf("twt/%s", yarn.FullVersion())

	// ErrBadRequest ...
	ErrBadRequest = errors.New("error: bad request")

	// ErrUnauthorized ...
	ErrUnauthorized = errors.New("error: authorization failed")

	// ErrServerError
	ErrServerError = errors.New("error: server error")
)

// Client ...
type Client struct {
	BaseURL   *url.URL
	Config    *Config
	UserAgent string
	Twter     types.Twter

	httpClient *http.Client
}

// NewClient ...
func NewClient(options ...Option) (*Client, error) {
	config := NewConfig()

	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	u, err := url.Parse(config.URI)
	if err != nil {
		return nil, err
	}
	baseURL := u.ResolveReference(&url.URL{Path: fmt.Sprintf("/api/v%d/", APIVersion)})

	cli := &Client{
		BaseURL:    baseURL,
		Config:     config,
		UserAgent:  DefaultUserAgent,
		Twter:      types.Twter{},
		httpClient: http.DefaultClient,
	}

	return cli, nil
}

func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	path = strings.TrimPrefix(path, "/")
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.Config.Token != "" {
		req.Header.Set("Token", c.Config.Token)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, v interface{}) error {
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusInternalServerError:
		return ErrServerError
	}

	err = json.NewDecoder(res.Body).Decode(v)

	return err
}

// Login ...
func (c *Client) Login(username, password string) (res types.AuthResponse, err error) {
	req, err := c.newRequest("POST", "/auth", types.AuthRequest{Username: username, Password: password})
	if err != nil {
		return types.AuthResponse{}, err
	}
	err = c.do(req, &res)
	return
}

// Post ...
func (c *Client) Post(text, as string) (res types.AuthResponse, err error) {
	req, err := c.newRequest("POST", "/post", types.PostRequest{Text: text, PostAs: as})
	if err != nil {
		return types.AuthResponse{}, err
	}
	err = c.do(req, &res)
	return
}

func (c *Client) GetAndSetTwter() error {
	if !c.Twter.IsZero() {
		return nil
	}

	res, err := c.Profile("")
	if err != nil {
		log.WithError(err).Error("error retrieving user profile")
		return err
	}
	c.Twter = types.Twter{Nick: "me", URL: res.Profile.URL}
	return nil
}

// Profile ...
func (c *Client) Profile(username string) (res types.ProfileResponse, err error) {
	var endpoint = "/profile"

	if username != "" {
		endpoint += "/" + username
	}

	req, err := c.newRequest("GET", endpoint, nil)
	if err != nil {
		return types.ProfileResponse{}, err
	}
	err = c.do(req, &res)
	return
}

// Timeline ...
func (c *Client) Timeline(page int) (res types.PagedResponse, err error) {
	if err := c.GetAndSetTwter(); err != nil {
		log.WithError(err).Error("unable to get or set our own Twter identity")
		return types.PagedResponse{}, nil
	}

	req, err := c.newRequest("POST", "/timeline", types.PagedRequest{Page: page})
	if err != nil {
		return types.PagedResponse{}, err
	}
	err = c.do(req, &res)
	return
}
