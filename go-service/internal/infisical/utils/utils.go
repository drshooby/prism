package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func GetInfisicalAccessToken(clientID, clientSecret, siteURL string) (string, error) {
	payload := url.Values{}
	payload.Set("clientId", clientID)
	payload.Set("clientSecret", clientSecret)

	loginURL := fmt.Sprintf("%s/api/v1/auth/universal-auth/login", siteURL)
	resp, err := http.PostForm(loginURL, payload)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.AccessToken, nil
}
