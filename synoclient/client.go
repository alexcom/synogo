package synoclient

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

// Client ...
type Client struct {
	Host     string
	Scheme   string
	Username string
	Password string
	Session  string
	Timeout  time.Duration
	Sid      string
}

// NewRequest ...
func (c *Client) NewRequest(method string, path string, params map[string]string) (*http.Request, error) {

	url := url.URL{
		Scheme: c.Scheme,
		Host:   c.Host,
		Path:   path,
	}

	query := url.Query()
	for param, value := range params {
		query.Set(param, value)
	}

	// pack _sid param to each request if logged-in
	if c.Sid != "" {
		query.Set("_sid", c.Sid)
	}

	url.RawQuery = query.Encode()
	//fmt.Printf("\nRequest: %s\n", url.String())

	req, err := http.NewRequest(method, url.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	return req, nil
}

// Do ...
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	timeout := time.Duration(c.Timeout * time.Second)
	httpClient := http.Client{
		Timeout: timeout,
	}
	resp, err := httpClient.Do(req)
	//fmt.Printf("\nResponse: %v\n", resp)
	if err != nil {
		return nil, &GenericError{desc: err.Error()}
	}

	if resp.StatusCode >= 400 {
		return nil, &GenericError{desc: resp.Status}
	}
	return resp, err
}

// Get ...
func (c *Client) Get(path string, params map[string]string) (string, error) {

	// assemble the request
	req, err := c.NewRequest("GET", path, params)
	if err != nil {
		return "", err
	}

	// make the call
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}

	// read response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic("Could not read response body")
	}
	defer resp.Body.Close()

	// assert common Synology API errors
	if err := c.AssertResponse(body); err != nil {
		return "", err
	}

	// was:
	//return responseData.(map[string]interface{})["data"], nil
	return string(body), nil

}

// AssertResponse ...
func (c *Client) AssertResponse(responseBody []byte) (err error) {
	var responseData interface{}
	err = json.Unmarshal(responseBody, &responseData)

	if err != nil {
		return err
	}
	success := responseData.(map[string]interface{})["success"].(bool)
	if success {
		return nil
	}

	errorBlock := responseData.(map[string]interface{})["error"]
	errorCode := int(errorBlock.(map[string]interface{})["code"].(float64))
	return &SynoError{code: errorCode}
}

func (c *Client) Login() (sid string, err error) {
	loginParams := map[string]string{
		"api":     "SYNO.API.Auth",
		"version": "2",
		"method":  "login",
		"account": c.Username,
		"passwd":  c.Password,
		"session": c.Session,
		"format":  "sid",
	}
	r, err := c.Get("webapi/auth.cgi", loginParams)
	if err != nil {
		return "", err
	}

	data := c.GetData(r)
	sid = data.(map[string]interface{})["sid"].(string)

	// set the sid field to pass with any subsequent request
	c.Sid = sid
	return sid, nil

}

func (c *Client) Logout() error {
	logoutParams := map[string]string{
		"api":     "SYNO.API.Auth",
		"version": "2",
		"method":  "logout",
		"session": c.Session,
	}

	_, err := c.Get("webapi/auth.cgi", logoutParams)
	if err != nil {
		return err
	}

	return nil
}

// get "data" object from json response
func (c *Client) GetData(data string) interface{} {
	var responseData interface{}
	json.Unmarshal([]byte(data), &responseData)
	return responseData.(map[string]interface{})["data"]
}
