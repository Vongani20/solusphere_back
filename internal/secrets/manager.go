package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// AppSecrets holds all application secrets
type AppSecrets struct {
	JWTSecret     string `json:"JWT_SECRET"`
	OpenAIAPIKey  string `json:"OPENAI_API_KEY"`
	OpenAIModel   string `json:"OPENAI_MODEL"`
	DBUser        string `json:"DB_USER"`
	DBPassword    string `json:"DB_PASSWORD"`
	DBHost        string `json:"DB_HOST"`
	DBPort        string `json:"DB_PORT"`
	DBName        string `json:"DB_NAME"`
	AWSAccessKey  string `json:"AWS_ACCESS_KEY_ID"`
	AWSSecretKey  string `json:"AWS_SECRET_ACCESS_KEY"`
	AWSRegion     string `json:"AWS_REGION"`
	AWSBucketName string `json:"AWS_BUCKET_NAME"`
	SMTPHost      string `json:"SMTP_HOST"`
	SMTPPort      string `json:"SMTP_PORT"`
	SMTPUsername  string `json:"SMTP_USERNAME"`
	SMTPPassword  string `json:"SMTP_PASSWORD"`
	SMTPFrom      string `json:"SMTP_FROM"`
}

var (
	instance *Manager
	once     sync.Once
)

// Manager handles AWS Secrets Manager operations
type Manager struct {
	client     *secretsmanager.Client
	secretName string
	region     string
	cache      *AppSecrets
	mu         sync.RWMutex
	lastFetch  time.Time
	ttl        time.Duration
}

// GetInstance returns a singleton instance of the secrets manager
func GetInstance(secretName, region string) (*Manager, error) {
	var err error
	once.Do(func() {
		var cfg aws.Config
		cfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(region),
		)
		if err != nil {
			err = fmt.Errorf("failed to load AWS config: %w", err)
			return
		}

		instance = &Manager{
			client:     secretsmanager.NewFromConfig(cfg),
			secretName: secretName,
			region:     region,
			ttl:        15 * time.Minute,
		}
	})
	return instance, err
}

// GetSecrets retrieves secrets with caching
func (m *Manager) GetSecrets(ctx context.Context) (*AppSecrets, error) {
	// Check cache
	m.mu.RLock()
	if m.cache != nil && time.Since(m.lastFetch) < m.ttl {
		defer m.mu.RUnlock()
		return m.cache, nil
	}
	m.mu.RUnlock()

	// Cache miss, fetch from AWS
	return m.refreshSecrets(ctx)
}

// refreshSecrets fetches fresh secrets from AWS
func (m *Manager) refreshSecrets(ctx context.Context) (*AppSecrets, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring lock
	if m.cache != nil && time.Since(m.lastFetch) < m.ttl {
		return m.cache, nil
	}

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(m.secretName),
	}

	result, err := m.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret value: %w", err)
	}

	var secrets AppSecrets
	err = json.Unmarshal([]byte(*result.SecretString), &secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse secret JSON: %w", err)
	}

	m.cache = &secrets
	m.lastFetch = time.Now()

	log.Println("Secrets refreshed successfully from AWS Secrets Manager")
	return &secrets, nil
}

// StartRefreshTicker periodically refreshes secrets
func (m *Manager) StartRefreshTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if _, err := m.refreshSecrets(ctx); err != nil {
					log.Printf("Failed to refresh secrets: %v", err)
				}
			}
		}
	}()
}
