package models

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	S3Client         *s3.Client
	RekognitionClient *rekognition.Client
	BucketName       string
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
	
	BucketName = bucketName

	log.Printf("✅ AWS clients initialized for region: %s, bucket: %s", region, bucketName)
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
	if S3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	// Convert []byte to io.Reader using bytes.NewReader
	_, err := S3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body), // FIXED: Convert []byte to io.Reader
	})
	
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}
	
	return nil
}