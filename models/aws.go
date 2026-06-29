package models

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

var (
	S3Client          *s3.Client
	RekognitionClient *rekognition.Client
	SNSClient         *sns.Client
	BucketName        string
	AWSRegionName     string
)

// InitAWS initializes AWS using environment variables (backward compatibility)
func InitAWS() error {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("AWS_REGION")
	bucketName := os.Getenv("AWS_BUCKET_NAME")

	return InitAWSWithSecrets(accessKey, secretKey, region, bucketName)
}

// InitAWSWithSecrets initializes AWS clients with provided secrets
func InitAWSWithSecrets(accessKey, secretKey, region, bucketName string) error {
	// Create custom credentials provider
	var credProvider aws.CredentialsProvider
	if accessKey != "" && secretKey != "" {
		credProvider = credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	}

	// Load AWS config
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if credProvider != nil {
		opts = append(opts, config.WithCredentialsProvider(credProvider))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Initialize S3 client
	S3Client = s3.NewFromConfig(cfg)

	// Initialize Rekognition client
	RekognitionClient = rekognition.NewFromConfig(cfg)

	// Initialize SNS client for password-reset SMS messages
	SNSClient = sns.NewFromConfig(cfg)

	BucketName = bucketName
	AWSRegionName = region

	log.Printf("✅ AWS clients initialized for region: %s, bucket: %s", region, bucketName)
	return nil
}

func PublishSMS(phoneNumber, message string) error {
	if SNSClient == nil {
		return fmt.Errorf("SNS client not initialized")
	}
	if strings.TrimSpace(phoneNumber) == "" {
		return fmt.Errorf("phone number is required")
	}

	_, err := SNSClient.Publish(context.Background(), &sns.PublishInput{
		PhoneNumber: aws.String(phoneNumber),
		Message:     aws.String(message),
	})
	if err != nil {
		return fmt.Errorf("failed to publish SMS with SNS: %w", err)
	}

	return nil
}

// GetS3Client returns the S3 client
func GetS3Client() *s3.Client {
	if S3Client == nil {
		log.Fatal("❌ S3 client not initialized. Call InitAWS first.")
	}
	return S3Client
}

// GetRekognitionClient returns the Rekognition client
func GetRekognitionClient() *rekognition.Client {
	if RekognitionClient == nil {
		log.Fatal("❌ Rekognition client not initialized. Call InitAWS first.")
	}
	return RekognitionClient
}

// GetBucketName returns the S3 bucket name
func GetBucketName() string {
	if BucketName == "" {
		log.Fatal("❌ Bucket name not set. Call InitAWS first.")
	}
	return BucketName
}

// UploadToS3 uploads a file to S3 (fixed version)
func UploadToS3(key string, body []byte) error {
	return UploadToS3WithContentType(key, body, "")
}

func UploadToS3WithContentType(key string, body []byte, contentType string) error {
	if S3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}
	if BucketName == "" {
		return fmt.Errorf("S3 bucket name is not configured")
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := S3Client.PutObject(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// DownloadFromS3 fetches an object from the configured bucket using AWS credentials.
func DownloadFromS3(key string) ([]byte, string, error) {
	if S3Client == nil {
		return nil, "", fmt.Errorf("S3 client not initialized")
	}
	if BucketName == "" {
		return nil, "", fmt.Errorf("S3 bucket name is not configured")
	}
	if strings.TrimSpace(key) == "" {
		return nil, "", fmt.Errorf("S3 key is required")
	}

	out, err := S3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to download from S3: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read S3 object body: %w", err)
	}

	contentType := ""
	if out.ContentType != nil {
		contentType = strings.ToLower(*out.ContentType)
	}
	return data, contentType, nil
}

func DeleteFromS3(key string) error {
	if S3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}
	if BucketName == "" {
		return fmt.Errorf("S3 bucket name is not configured")
	}
	if strings.TrimSpace(key) == "" {
		return nil
	}

	_, err := S3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

func S3ObjectURL(key string) string {
	escapedKey := escapeS3Key(key)
	if AWSRegionName == "" {
		return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", BucketName, escapedKey)
	}

	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", BucketName, AWSRegionName, escapedKey)
}

func S3KeyFromObjectURL(rawURL string) (string, bool) {
	if strings.HasPrefix(rawURL, "s3://") {
		withoutScheme := strings.TrimPrefix(rawURL, "s3://")
		parts := strings.SplitN(withoutScheme, "/", 2)
		if len(parts) == 2 && parts[0] == BucketName {
			return parts[1], true
		}
		return "", false
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	if !strings.HasPrefix(parsed.Host, BucketName+".s3.") && parsed.Host != BucketName+".s3.amazonaws.com" {
		return "", false
	}

	key, err := url.PathUnescape(strings.TrimPrefix(parsed.Path, "/"))
	if err != nil || key == "" {
		return "", false
	}
	return key, true
}

func escapeS3Key(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
