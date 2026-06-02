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
// For backward compatibility
func NewRekognitionService() *RekognitionService {
	return NewRekognitionServiceWithCredentials(
		os.Getenv("AWS_ACCESS_KEY_ID"),
		os.Getenv("AWS_SECRET_ACCESS_KEY"),
		os.Getenv("AWS_REGION"),
		os.Getenv("AWS_BUCKET_NAME"),
	)
}

// NewRekognitionServiceWithCredentials creates a new Rekognition service with provided credentials
// This should be called after loading secrets from AWS Secrets Manager
func NewRekognitionServiceWithCredentials(accessKey, secretKey, region, bucket string) *RekognitionService {
	if bucket == "" {
		log.Fatal("❌ AWS_BUCKET_NAME is not set")
	}
	if region == "" {
		log.Fatal("❌ AWS_REGION is not set")
	}

	// Create custom credentials provider if keys are provided
	var credProvider aws.CredentialsProvider
	if accessKey != "" && secretKey != "" {
		credProvider = credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	}

	// Load AWS Configuration
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if credProvider != nil {
		opts = append(opts, config.WithCredentialsProvider(credProvider))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		log.Fatalf("❌ Failed to load AWS config: %v", err)
	}

	// Create Rekognition Client
	client := rekognition.NewFromConfig(cfg)

	log.Printf("✅ Rekognition service initialized for region: %s, bucket: %s", region, bucket)

	return &RekognitionService{
		Client:    client,
		S3Bucket:  bucket,
		AWSRegion: region,
	}
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
// Useful when secrets are rotated
func RefreshRekognitionService(accessKey, secretKey, region, bucket string) {
	// Reset the once so we can reinitialize
	rekognitionOnce = sync.Once{}
	InitRekognitionService(accessKey, secretKey, region, bucket)
	log.Println("🔄 Rekognition service refreshed with new credentials")
}

// ensureCollectionExists checks if a collection exists and creates it if it doesn't.
func (r *RekognitionService) ensureCollectionExists(collectionID string) error {
	// Check if the collection exists
	_, err := r.Client.DescribeCollection(context.TODO(), &rekognition.DescribeCollectionInput{
		CollectionId: aws.String(collectionID),
	})

	if err == nil {
		log.Printf("✅ Collection %s already exists", collectionID)
		return nil // Collection already exists
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

	// If it's any other error (AccessDenied, etc.), return it
	return fmt.Errorf("failed to describe collection %s: %w", collectionID, err)
}

// IndexFaces uploads the S3 object to the Rekognition Collection for indexing.
func (r *RekognitionService) IndexFaces(collectionID string, s3Key string, externalImageID string) (*rekognition.IndexFacesOutput, error) {
	log.Printf("📸 Indexing face for user %s from S3: %s/%s", externalImageID, r.S3Bucket, s3Key)

	// 1. Ensure the Collection exists
	if err := r.ensureCollectionExists(collectionID); err != nil {
		return nil, fmt.Errorf("failed to ensure collection exists: %w", err)
	}

	// 2. Prepare the IndexFaces input
	input := &rekognition.IndexFacesInput{
		CollectionId: aws.String(collectionID),
		Image: &types.Image{
			S3Object: &types.S3Object{
				Bucket: aws.String(r.S3Bucket),
				Name:   aws.String(s3Key),
			},
		},
		ExternalImageId: aws.String(externalImageID),
		MaxFaces:        aws.Int32(1),
		QualityFilter:   "AUTO", // Automatically filter low-quality faces
		DetectionAttributes: []types.Attribute{
			types.AttributeDefault, // Get default attributes
		},
	}

	// 3. Call the Rekognition API
	resp, err := r.Client.IndexFaces(context.TODO(), input)
	if err != nil {
		log.Printf("❌ Rekognition IndexFaces API error: %v", err)
		return nil, fmt.Errorf("rekognition index faces API error: %w", err)
	}

	// Log results
	log.Printf("📊 IndexFaces results: %d faces indexed, %d faces unindexed",
		len(resp.FaceRecords), len(resp.UnindexedFaces))

	if len(resp.FaceRecords) == 0 {
		// Log reasons for unindexed faces
		for i, reason := range resp.UnindexedFaces {
			log.Printf("⚠️ Unindexed face %d: Reasons: %+v", i, reason.Reasons)
			if reason.FaceDetail != nil {
				log.Printf("   Confidence: %.2f, Quality: %+v", 
					*reason.FaceDetail.Confidence, reason.FaceDetail.Quality)
			}
		}
		return nil, fmt.Errorf("no face indexed. Unindexed reasons: %+v", resp.UnindexedFaces)
	}

	// Log successful indexing
	for i, face := range resp.FaceRecords {
		log.Printf("✅ Face %d indexed with ID: %s, Confidence: %.2f", 
			i+1, *face.Face.FaceId, *face.Face.Confidence)
	}

	return resp, nil
}

// SearchCollectionByImage searches a Rekognition Collection using a new face image (bytes).
func (r *RekognitionService) SearchCollectionByImage(
	collectionID string,
	faceBytes []byte,
) (*rekognition.SearchFacesByImageOutput, error) {
	
	log.Printf("🔍 Searching for face in collection: %s", collectionID)

	// First ensure collection exists (will create if not, but for search it should exist)
	if err := r.ensureCollectionExists(collectionID); err != nil {
		log.Printf("⚠️ Warning when checking collection: %v", err)
		// Continue anyway, the search will fail if collection doesn't exist
	}

	input := &rekognition.SearchFacesByImageInput{
		CollectionId: aws.String(collectionID),
		Image: &types.Image{
			Bytes: faceBytes,
		},
		FaceMatchThreshold: aws.Float32(80.0), // Only return matches >= 80%
		MaxFaces:           aws.Int32(1),       // Return top match only
	}

	resp, err := r.Client.SearchFacesByImage(context.TODO(), input)

	if err != nil {
		// Check for specific error types
		var invalidParamErr *types.InvalidParameterException
		var accessDeniedErr *types.AccessDeniedException
		var resourceNotFoundErr *types.ResourceNotFoundException

		switch {
		case errors.As(err, &invalidParamErr):
			log.Printf("❌ Invalid parameter in search: %v", err)
		case errors.As(err, &accessDeniedErr):
			log.Printf("❌ Access denied to Rekognition: %v", err)
		case errors.As(err, &resourceNotFoundErr):
			log.Printf("❌ Collection not found: %s", collectionID)
		default:
			log.Printf("❌ Rekognition SearchFacesByImage failed: %v", err)
		}
		
		return nil, fmt.Errorf("rekognition search faces by image API error: %w", err)
	}

	// Log search results
	if len(resp.FaceMatches) > 0 {
		for i, match := range resp.FaceMatches {
			log.Printf("✅ Match %d: Face ID: %s, Similarity: %.2f%%", 
				i+1, *match.Face.FaceId, *match.Similarity)
		}
	} else {
		log.Printf("ℹ️ No matches found in collection %s", collectionID)
	}

	return resp, nil
}

// DeleteFaces deletes faces from a collection
func (r *RekognitionService) DeleteFaces(collectionID string, faceIDs []string) (*rekognition.DeleteFacesOutput, error) {
	log.Printf("🗑️ Deleting %d faces from collection %s", len(faceIDs), collectionID)

	input := &rekognition.DeleteFacesInput{
		CollectionId: aws.String(collectionID),
		FaceIds:      faceIDs,
	}

	resp, err := r.Client.DeleteFaces(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to delete faces: %w", err)
	}

	log.Printf("✅ Deleted %d faces successfully", len(resp.DeletedFaces))
	return resp, nil
}

// ListCollections lists all Rekognition collections
func (r *RekognitionService) ListCollections() (*rekognition.ListCollectionsOutput, error) {
	input := &rekognition.ListCollectionsInput{
		MaxResults: aws.Int32(100),
	}

	resp, err := r.Client.ListCollections(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	log.Printf("📋 Found %d collections", len(resp.CollectionIds))
	return resp, nil
}

// DeleteCollection deletes a collection and all faces in it
func (r *RekognitionService) DeleteCollection(collectionID string) error {
	log.Printf("🗑️ Deleting collection: %s", collectionID)

	input := &rekognition.DeleteCollectionInput{
		CollectionId: aws.String(collectionID),
	}

	_, err := r.Client.DeleteCollection(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	log.Printf("✅ Collection %s deleted successfully", collectionID)
	return nil
}

// GetCollectionInfo gets detailed information about a collection
func (r *RekognitionService) GetCollectionInfo(collectionID string) (*rekognition.DescribeCollectionOutput, error) {
	input := &rekognition.DescribeCollectionInput{
		CollectionId: aws.String(collectionID),
	}

	resp, err := r.Client.DescribeCollection(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe collection: %w", err)
	}

	log.Printf("📊 Collection %s: %d faces, created: %v", 
		collectionID, *resp.FaceCount, resp.CreationTimestamp)
	return resp, nil
}