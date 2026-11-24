package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"rune/internal/auth"
)

type Client struct {
	BaseURL string
	Token   string
}

type SecretVersion struct {
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	RotatedAt string `json:"rotated_at"`
}

type SecretMeta struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	RotatedAt string `json:"rotated_at,omitempty"`
	TTL       string `json:"ttl,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

func New(baseURL string, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
	}
}

// CORE VAULT MANAGEMENT

func (c *Client) Unseal(passphrase string) error {
	body := map[string]string{
		"passphrase": passphrase,
	}

	data, _ := json.Marshal(body)
	resp, err := http.Post(c.BaseURL+"/unseal", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error: %s", string(bodyBytes))
	}

	return nil

}
func (c *Client) Seal() error {
	req, _ := http.NewRequest("POST", c.BaseURL+"/seal", nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	_, err := http.DefaultClient.Do(req)
	return err
}
func (c *Client) Put(key, value, namespace string) error {

	body := map[string]string{
		"key":       key,
		"value":     value,
		"namespace": namespace,
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", c.BaseURL+"/secret/put", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(bodyBytes))
	}
	return nil

}
func (c *Client) Get(key, namespace string) (string, error) {

	req, err := http.NewRequest("GET", c.BaseURL+"/secret/get?key="+key+"&namespace="+namespace, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(string(body))
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	return result["value"], nil

}
func (c *Client) List(namespace string) ([]string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/secret/list?namespace="+namespace, nil)
	if err != nil {
		return nil, err
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(string(bodyBytes))
	}

	var keys []string
	err = json.NewDecoder(resp.Body).Decode(&keys)
	if err != nil {
		return nil, err
	}

	return keys, nil
}
func (c *Client) Delete(key string, namespace string) error {
	body := map[string]string{
		"key":       key,
		"namespace": namespace,
	}

	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", c.BaseURL+"/secret/delete", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(b))
	}

	return nil
}
func (c *Client) Rotate(key string, namespace string) (string, error) {
	url := c.BaseURL + "/secret/rotate?key=" + url.QueryEscape(key) + "&namespace=" + url.QueryEscape(namespace)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%s", body)
	}

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	return result["value"], nil

}
func (c *Client) Status() (string, error) {

	resp, err := http.Get(c.BaseURL + "/status")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil

}

// TOKEN MANAGEMENT

func (c *Client) CreateToken(name string, namespaces []auth.NamespacePermission) (string, auth.TokenRecord, error) {
	body := map[string]interface{}{
		"name":       name,
		"namespaces": namespaces,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.BaseURL+"/token/create", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", auth.TokenRecord{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", auth.TokenRecord{}, fmt.Errorf(string(b))
	}
	var result struct {
		Token  string           `json:"token"`
		Record auth.TokenRecord `json:"record"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Token, result.Record, nil
}
func (c *Client) CreateSystemToken(name string, namespace string) (string, auth.TokenRecord, error) {
	body := map[string]string{
		"name":      name,
		"namespace": namespace,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.BaseURL+"/token/create-system", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", auth.TokenRecord{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", auth.TokenRecord{}, fmt.Errorf(string(b))
	}
	var result struct {
		Token  string           `json:"token"`
		Record auth.TokenRecord `json:"record"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Token, result.Record, nil
}

func (c *Client) ListTokens() ([]auth.TokenRecord, error) {

	req, _ := http.NewRequest("GET", c.BaseURL+"/token/list", nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(string(b))
	}
	var tokens []auth.TokenRecord
	json.NewDecoder(resp.Body).Decode(&tokens)
	return tokens, nil

}
func (c *Client) RevokeToken(id string) error {

	body := map[string]string{
		"id": id,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.BaseURL+"/token/revoke", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(b))
	}
	return nil

}

// NAMESPACES

func (c *Client) CreateNamespace(name string) error {

	body := map[string]string{
		"name": name,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/namespace/create", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(b))
	}

	return nil
}
func (c *Client) ListNamespace() ([]string, error) {

	req, err := http.NewRequest("GET", c.BaseURL+"/namespace/list", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(string(bodyBytes))
	}

	var namespaces []string
	if err := json.NewDecoder(resp.Body).Decode(&namespaces); err != nil {
		return nil, err
	}

	return namespaces, nil
}
func (c *Client) DeleteNamespace(name string) error {
	body := map[string]string{
		"name": name,
	}

	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", c.BaseURL+"/namespace/delete", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(b))
	}

	return nil
}

func (c *Client) GetAll(namespace string) (map[string]string, error) {
	reqURL := fmt.Sprintf("%s/secret/all?namespace=%s", c.BaseURL, url.QueryEscape(namespace))
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", body)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// VERSIONING
func (c *Client) GetVersion(key string, namespace string, version int) (string, error) {

	reqUrl := fmt.Sprintf("%s/secret/version?key=%s&namespace=%s&version=%d", c.BaseURL, url.QueryEscape(key),
		url.QueryEscape(namespace), version)

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%s", body)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result["value"], nil
}
func (c *Client) SetTTL(key, namespace, ttl string) error {
	body := map[string]string{
		"key":       key,
		"namespace": namespace,
		"ttl":       ttl,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.BaseURL+"/secret/ttl", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", b)
	}
	return nil
}

func (c *Client) ListMeta(namespace string) ([]SecretMeta, error) {
	reqURL := fmt.Sprintf("%s/secret/meta?namespace=%s", c.BaseURL, url.QueryEscape(namespace))
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", b)
	}
	var metas []SecretMeta
	if err := json.NewDecoder(resp.Body).Decode(&metas); err != nil {
		return nil, err
	}
	return metas, nil
}

func (c *Client) ListVersions(key string, namespace string) ([]SecretVersion, error) {

	reqURL := fmt.Sprintf("%s/secret/history?key=%s&namespace=%s", c.BaseURL,
		url.QueryEscape(key), url.QueryEscape(namespace),
	)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", body)
	}

	var versions []SecretVersion

	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, err
	}

	return versions, nil
}
