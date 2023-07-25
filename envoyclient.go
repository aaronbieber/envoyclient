package envoyclient

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/boltdb/bolt"
)

type EnvoyClient struct {
	Cache  envoyCache
	Config Config
}

type Config struct {
	Email         string
	Password      string
	EnvoySerialNo string
	EnvoyIP       string
}

type envoyCache struct {
	EnvoyToken          string
	EnvoyTokenExpiresAt time.Time
}

type enphaseLoginResponse struct {
	SessionId string `json:"session_id"`
}

type productionData struct {
	ProductionWattsNow  float64
	ConsumptionWattsNow float64
}

func (e *EnvoyClient) readCache() error {
	db, err := bolt.Open("envoy.db", 0600, nil)
	if err != nil {
		return fmt.Errorf("error opening cache database: %v", err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("envoy"))
		if err != nil {
			return fmt.Errorf("could not create bucket: %s", err)
		}

		b := tx.Bucket([]byte("envoy"))

		envoyToken := b.Get([]byte("envoyToken"))
		if envoyToken != nil {
			e.Cache.EnvoyToken = string(envoyToken)
		}

		envoyTokenExpiresAtBinary := b.Get([]byte("envoyTokenExpiresAt"))
		if envoyTokenExpiresAtBinary != nil {
			envoyTokenExpiresAt := time.Time{}
			envoyTokenExpiresAt.UnmarshalBinary(envoyTokenExpiresAtBinary)
			e.Cache.EnvoyTokenExpiresAt = envoyTokenExpiresAt
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (e *EnvoyClient) writeCache() error {
	db, err := bolt.Open("envoy.db", 0600, nil)
	if err != nil {
		return fmt.Errorf("error opening cache database: %w", err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("envoy"))

		err = b.Put([]byte("envoyToken"), []byte(e.Cache.EnvoyToken))
		if err != nil {
			return fmt.Errorf("could not write envoyToken: %w", err)
		}

		expireBytes, err := e.Cache.EnvoyTokenExpiresAt.MarshalBinary()
		if err != nil {
			return fmt.Errorf("could not marshal envoyTokenExpiresAt: %w", err)
		}

		err = b.Put([]byte("envoyTokenExpiresAt"), expireBytes)
		if err != nil {
			return fmt.Errorf("could not write envoyTokenExpiresAt: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error writing cache: %w", err)
	}

	return nil
}

func (e *EnvoyClient) requestNewEnvoyToken() (string, error) {
	sessionId, err := e.getEnphaseSessionId()
	if err != nil {
		return "", err
	}

	reqData := map[string]string{
		"session_id": sessionId,
		"serial_num": e.Config.EnvoySerialNo,
		"username":   e.Config.Email}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("error marshalling json: %v", err)
	}

	resp, err := http.Post(
		"https://entrez.enphaseenergy.com/tokens",
		"application/json",
		strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}

	envoyToken, _ := io.ReadAll(resp.Body)

	return string(envoyToken), nil
}

func (e *EnvoyClient) getEnvoyToken() (string, error) {
	if e.Cache.EnvoyToken == "" || e.Cache.EnvoyTokenExpiresAt.Before(time.Now()) {
		var err error
		e.Cache.EnvoyToken, err = e.requestNewEnvoyToken()
		if err != nil {
			return "", fmt.Errorf("error requesting new envoy token: %v", err)
		}
		e.Cache.EnvoyTokenExpiresAt = time.Now().Add(time.Hour * 24)

		err = e.writeCache()
		if err != nil {
			return "", fmt.Errorf("error writing cache: %v", err)
		}
	}

	return e.Cache.EnvoyToken, nil
}

func (e *EnvoyClient) getEnphaseSessionId() (string, error) {
	reqData := url.Values{}
	reqData.Set("user[email]", e.Config.Email)
	reqData.Set("user[password]", e.Config.Password)

	client := &http.Client{}
	req, _ := http.NewRequest(
		"POST",
		"https://enlighten.enphaseenergy.com/login/login.json",
		strings.NewReader(reqData.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	var response enphaseLoginResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling response body: %v", err)
	}

	return response.SessionId, nil
}

func (e *EnvoyClient) GetProductionData() (productionData, error) {
	envoyToken, err := e.getEnvoyToken()
	if err != nil {
		return productionData{}, fmt.Errorf("error getting envoy token: %v", err)
	}

	req, _ := http.NewRequest(
		"GET",
		"https://"+e.Config.EnvoyIP+"/production.json",
		nil)
	req.Header.Add("Authorization", "Bearer "+envoyToken)

	// Envoy uses a self-signed cert, c'est la vie
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Do(req)
	if err != nil {
		return productionData{}, fmt.Errorf("error making request: %v", err)
	}

	var resData map[string]interface{}
	jsonData, _ := io.ReadAll(res.Body)
	json.Unmarshal(jsonData, &resData)

	consumption := resData["consumption"].([]interface{})
	production := resData["production"].([]interface{})

	var productionData productionData

	for _, v := range consumption {
		vals := v.(map[string]interface{})
		if vals["measurementType"] == "total-consumption" {
			productionData.ConsumptionWattsNow = vals["wNow"].(float64)
		}
	}

	for _, v := range production {
		vals := v.(map[string]interface{})
		if vals["measurementType"] == "production" {
			productionData.ProductionWattsNow = vals["wNow"].(float64)
		}
	}

	return productionData, nil
}

func NewClient(c Config) (*EnvoyClient, error) {
	client := &EnvoyClient{}
	client.Config = c
	err := client.readCache()
	if err != nil {
		return nil, fmt.Errorf("error reading cache: %v", err)
	}

	return client, nil
}
