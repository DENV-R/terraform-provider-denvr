package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client : petit client HTTP de l'API REST DENV-R (authentification par jeton Bearer).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient construit un client. baseURL sans slash final ; le préfixe /api/v1 est ajouté par appel.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// apiError expose le message d'erreur JSON renvoyé par l'API ({"error": "..."}).
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API DENV-R (HTTP %d) : %s", e.Status, e.Message)
}

// do émet une requête et décode la réponse JSON dans out (peut être nil).
// Retourne une *apiError si le statut HTTP est >= 400.
func (c *Client) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, c.BaseURL+"/api/v1"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(raw))
		var env struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &env) == nil && env.Error != "" {
			msg = env.Error
		}
		return &apiError{Status: resp.StatusCode, Message: msg}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("réponse JSON illisible : %w", err)
		}
	}
	return nil
}
