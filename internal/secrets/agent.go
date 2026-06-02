package secrets

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// SecretsManagerAgent handles secret retrieval via the AWS Secrets Manager Agent
type SecretsManagerAgent struct {
	endpoint string
	token    string
}

// NewSecretsManagerAgent creates a new Secrets Manager Agent client
func NewSecretsManagerAgent() (*SecretsManagerAgent, error) {
	endpoint := os.Getenv("SECRETS_AGENT_URL")
	if endpoint == "" {
		endpoint = "http://localhost:2773"
	}

	// Read SSRF token from file (created by Secrets Manager Agent)
	tokenPath := "/var/run/awssmatoken"
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		// If token file doesn't exist, try environment variable
		token = []byte(os.Getenv("AWS_SESSION_TOKEN"))
		if len(token) == 0 {
			// For testing without agent, return a dummy agent
			return &SecretsManagerAgent{
				endpoint: endpoint,
				token:    "test-token",
			}, nil
		}
	}

	return &SecretsManagerAgent{
		endpoint: endpoint,
		token:    string(token),
	}, nil
}

// GetSecret retrieves a secret from the Secrets Manager Agent
func (a *SecretsManagerAgent) GetSecret(secretID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/secretsmanager/get?secretId=%s", a.endpoint, secretID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-Aws-Parameters-Secrets-Token", a.token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		SecretString string `json:"SecretString"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	var secret map[string]interface{}
	if err := json.Unmarshal([]byte(result.SecretString), &secret); err != nil {
		return nil, fmt.Errorf("failed to parse secret string: %v", err)
	}

	return secret, nil
}
