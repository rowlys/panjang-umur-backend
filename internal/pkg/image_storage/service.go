package image_storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appConfig "github.com/rowlys/panjang-umur-backend/internal/config"
)

type Service interface {
	GeneratePresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	PromoteFile(ctx context.Context, sourceKey, destinationKey string) error
	DeleteFile(ctx context.Context, key string) error
	GetPublicURL(key string) string
}

type service struct {
	client     *s3.Client
	presigner  *s3.PresignClient
	bucketName string
	bucketURL  string
}

func NewService() Service {
	accountID := appConfig.GetEnv("R2_ACCOUNT_ID", "")
	endpointOverride := appConfig.GetEnv("R2_ENDPOINT", "")
	accessKeyID := appConfig.GetEnv("R2_ACCESS_KEY_ID", "")
	secretAccessKey := appConfig.GetEnv("R2_SECRET_ACCESS_KEY", "")
	bucketName := appConfig.GetEnv("R2_BUCKET_NAME", "panjang-umur")
	bucketURL := appConfig.GetEnv("R2_BUCKET_URL", "")

	if accessKeyID == "" || secretAccessKey == "" || bucketName == "" || bucketURL == "" {
		log.Fatalf("R2 credentials or bucket information is missing. Please check the environment variables.")
	}

	var apiEndpoint string
	if endpointOverride != "" {
		apiEndpoint = endpointOverride
	} else if accountID != "" {
		apiEndpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	} else {
		log.Fatal("Either R2_ACCOUNT_ID or R2_ENDPOINT must be provided for S3 API operations")
	}

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: apiEndpoint,
		}, nil
	})

	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithEndpointResolverWithOptions(r2Resolver),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		awsConfig.WithRegion("auto"),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS configuration: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	presigner := s3.NewPresignClient(client)

	return &service{
		client:     client,
		presigner:  presigner,
		bucketName: bucketName,
		bucketURL:  bucketURL,
	}
}

func (s *service) GeneratePresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}

func (s *service) PromoteFile(ctx context.Context, sourceKey, destinationKey string) error {
	copySource := fmt.Sprintf("%v/%v", s.bucketName, sourceKey)
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucketName),
		CopySource: aws.String(copySource),
		Key:        aws.String(destinationKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object in R2: %w", err)
	}

	return s.DeleteFile(ctx, sourceKey)
}

func (s *service) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object from R2: %w", err)
	}
	return nil
}

func (s *service) GetPublicURL(key string) string {
	return fmt.Sprintf("%s/%s", s.bucketURL, key)
}
