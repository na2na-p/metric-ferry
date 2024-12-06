package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type EnvValues struct {
	SwitchBotToken        string `required:"true" split_words:"true"`
	SwitchBotClientSecret string `required:"true" split_words:"true"`
	Co2DeviceID           string `required:"true" split_words:"true"`

	APIKey  string `required:"true" split_words:"true"`
	PushURL string `required:"true" split_words:"true"`
}

type MeterProCO2Status struct {
	Temperature float64
	Battery     int
	Humidity    int
	CO2         int
}

func main() {
	var ev EnvValues
	if err := envconfig.Process("", &ev); err != nil {
		log.Fatal(err.Error())
	}

	status, err := getMeterProCO2Status(&ev)
	if err != nil {
		fmt.Println("Error:", err)
		log.Fatal(err)
	}

	metrics, err := formatMetrics(status, ev.Co2DeviceID)
	if err != nil {
		fmt.Println("Error formatting metrics:", err)
		log.Fatal(err)
	}

	err = sendMetrics(metrics, ev)
	if err != nil {
		fmt.Println("Error sending metrics:", err)
		log.Fatal(err)
	}

	log.Println("Metrics sent successfully")
}

func sendMetrics(metrics string, envValues EnvValues) error {
	apiKey := envValues.APIKey
	url := envValues.PushURL

	bearer := "Bearer " + apiKey

	fmt.Println(metrics)
	byteStr := []byte(metrics)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(byteStr))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", bearer)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	} else {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("received non-2xx response: %d, body: %s", resp.StatusCode, string(body))
	}
}

func formatMetrics(status *MeterProCO2Status, deviceID string) (string, error) {
	var metrics bytes.Buffer

	_, err := fmt.Fprintf(&metrics, "meterproco2_status,device_id=%s temperature=%f\n", deviceID, status.Temperature)
	if err != nil {
		return "", err
	}
	_, err = fmt.Fprintf(&metrics, "meterproco2_status,device_id=%s battery=%d\n", deviceID, status.Battery)
	if err != nil {
		return "", err
	}
	_, err = fmt.Fprintf(&metrics, "meterproco2_status,device_id=%s humidity=%d\n", deviceID, status.Humidity)
	if err != nil {
		return "", err
	}
	_, err = fmt.Fprintf(&metrics, "meterproco2_status,device_id=%s co2=%d\n", deviceID, status.CO2)
	if err != nil {
		return "", err
	}

	return metrics.String(), nil
}

func generateSignature(t int64, token, secret, nonce string) (string, error) {
	data := fmt.Sprintf("%s%d%s", token, t, nonce)
	h := hmac.New(sha256.New, []byte(secret))
	if _, err := h.Write([]byte(data)); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func getMeterProCO2Status(envValues *EnvValues) (*MeterProCO2Status, error) {
	url := fmt.Sprintf("https://api.switch-bot.com/v1.1/devices/%s/status", envValues.Co2DeviceID)
	nonce := "nonce"
	t := time.Now().UnixMilli()
	signature, err := generateSignature(t, envValues.SwitchBotToken, envValues.SwitchBotClientSecret, nonce)
	if err != nil {
		fmt.Println("Error generating signature:", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("sign", signature)
	req.Header.Set("nonce", nonce)
	req.Header.Set("t", fmt.Sprintf("%d", t))
	req.Header.Set("Authorization", envValues.SwitchBotToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		StatusCode int `json:"statusCode"`
		Body       struct {
			Temperature float64 `json:"temperature"`
			Battery     int     `json:"battery"`
			Humidity    int     `json:"humidity"`
			CO2         int     `json:"CO2"`
		} `json:"body"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return &MeterProCO2Status{
		Temperature: result.Body.Temperature,
		Battery:     result.Body.Battery,
		Humidity:    result.Body.Humidity,
		CO2:         result.Body.CO2,
	}, nil
}
