package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
)

var (
	rekognitionInstance *RekognitionService
	rekognitionOnce     sync.Once
)

type RekognitionService struct {
	Client    *rekognition.Client
	S3Bucket  string
	AWSRegion string
}

// NewRekognitionService creates a new Rekognition service using environment variables
func NewRekognitionService() *RekognitionService {
	return NewRekognitionServiceWithCredentials(
		os.Getenv("AWS_ACCESS_KEY_ID"),
		os.Getenv("AWS_SECRET_ACCESS_KEY"),
		os.Getenv("AWS_REGION"),
		os.Getenv("AWS_BUCKET_NAME"),
	)
}

// NewRekognitionServiceWithCredentials creates a new Rekognition service with provided credentials
func NewRekognitionServiceWithCredentials(accessKey, secretKey, region, bucket string) *RekognitionService {
	if bucket == "" {
		log.Fatal("❌ AWS_BUCKET_NAME is not set")
	}
	if region == "" {
		log.Fatal("❌ AWS_REGION is not set")
	}
	if accessKey == "" || secretKey == "" {
		log.Fatal("❌ AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}

	log.Println("🔧 Initializing Rekognition service with provided credentials")
	log.Printf("📊 Region: %s, Bucket: %s", region, bucket)

	// Create static credentials provider
	credProvider := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	// Load AWS Configuration with explicit credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credProvider),
	)
	if err != nil {
		log.Fatalf("❌ Failed to load AWS config: %v", err)
	}

	// Create Rekognition Client
	client := rekognition.NewFromConfig(cfg)

	log.Printf("✅ Rekognition service initialized successfully")
	log.Printf("✅ Client ready for region: %s", cfg.Region)

	return &RekognitionService{
		Client:    client,
		S3Bucket:  bucket,
		AWSRegion: region,
	}
}

// ensureCollectionExists checks if a collection exists and creates it if it doesn't.
func (r *RekognitionService) ensureCollectionExists(collectionID string) error {
	// Check if the collection exists
	_, err := r.Client.DescribeCollection(context.TODO(), &rekognition.DescribeCollectionInput{
		CollectionId: aws.String(collectionID),
	})

	if err == nil {
		log.Printf("✅ Collection %s already exists", collectionID)
		return nil
	}

	// Check if the error is ResourceNotFoundException
	var nfErr *types.ResourceNotFoundException
	if errors.As(err, &nfErr) {
		// Collection doesn't exist, create it
		log.Printf("📁 Collection %s not found. Creating it now...", collectionID)

		createInput := &rekognition.CreateCollectionInput{
			CollectionId: aws.String(collectionID),
		}

		createResult, createErr := r.Client.CreateCollection(context.TODO(), createInput)
		if createErr != nil {
			return fmt.Errorf("failed to create collection %s: %w", collectionID, createErr)
		}

		log.Printf("✅ Collection %s created successfully. ARN: %s", collectionID, *createResult.CollectionArn)
		return nil
	}

	return fmt.Errorf("failed to describe collection %s: %w", collectionID, err)
}

// SearchCollectionByImage searches a Rekognition Collection using a new face image (bytes).
func (r *RekognitionService) SearchCollectionByImage(
	collectionID string,
	faceBytes []byte,
) (*rekognition.SearchFacesByImageOutput, error) {

	log.Printf("🔍 Searching for face in collection: %s", collectionID)
	log.Printf("📸 Image size: %d bytes", len(faceBytes))

	// Ensure collection exists (will create if not)
	if err := r.ensureCollectionExists(collectionID); err != nil {
		log.Printf("⚠️ Warning when checking collection: %v", err)
	}

	input := &rekognition.SearchFacesByImageInput{
		CollectionId: aws.String(collectionID),
		Image: &types.Image{
			Bytes: faceBytes,
		},
		FaceMatchThreshold: aws.Float32(90.0),
		MaxFaces:           aws.Int32(5),
	}

	resp, err := r.Client.SearchFacesByImage(context.TODO(), input)

	if err != nil {
		log.Printf("❌ Rekognition SearchFacesByImage failed: %v", err)
		return nil, fmt.Errorf("rekognition search faces by image API error: %w", err)
	}

	log.Printf("✅ Found %d face matches", len(resp.FaceMatches))
	return resp, nil
}

// IndexFaces indexes a face in the collection
func (r *RekognitionService) IndexFaces(collectionID string, imageBytes []byte, externalImageID string) (*rekognition.IndexFacesOutput, error) {
	log.Printf("📸 Indexing face for user %s", externalImageID)

	if err := r.ensureCollectionExists(collectionID); err != nil {
		return nil, fmt.Errorf("failed to ensure collection exists: %w", err)
	}

	input := &rekognition.IndexFacesInput{
		CollectionId: aws.String(collectionID),
		Image: &types.Image{
			Bytes: imageBytes,
		},
		ExternalImageId: aws.String(externalImageID),
		MaxFaces:        aws.Int32(1),
		QualityFilter:   types.QualityFilterAuto,
		DetectionAttributes: []types.Attribute{
			types.AttributeDefault,
		},
	}

	resp, err := r.Client.IndexFaces(context.TODO(), input)
	if err != nil {
		log.Printf("❌ Rekognition IndexFaces error: %v", err)
		return nil, fmt.Errorf("rekognition index faces API error: %w", err)
	}

	log.Printf("✅ Indexed %d faces", len(resp.FaceRecords))
	return resp, nil
}

// GetInstance returns a singleton instance of RekognitionService
func GetRekognitionInstance() *RekognitionService {
	if rekognitionInstance == nil {
		log.Fatal("❌ Rekognition service not initialized. Call InitRekognitionService first.")
	}
	return rekognitionInstance
}

// InitRekognitionService initializes the singleton Rekognition service with credentials
func InitRekognitionService(accessKey, secretKey, region, bucket string) {
	rekognitionOnce.Do(func() {
		rekognitionInstance = NewRekognitionServiceWithCredentials(accessKey, secretKey, region, bucket)
	})
}

// RefreshRekognitionService refreshes the Rekognition service with new credentials
func RefreshRekognitionService(accessKey, secretKey, region, bucket string) {
	rekognitionOnce = sync.Once{}
	InitRekognitionService(accessKey, secretKey, region, bucket)
	log.Println("🔄 Rekognition service refreshed with new credentials")
}
