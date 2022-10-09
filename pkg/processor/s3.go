package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const FINAL_COUNT_PATH = "/tmp/objectCount"

func connectToS3(ctx context.Context) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(S3_REGION))
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg), nil
}

func s3Uploader(region string) (*s3manager.Uploader, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return nil, err
	}
	return s3manager.NewUploader(sess), nil
}

func uploadToS3(ctx context.Context, uploader *s3manager.Uploader, bucket, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: &bucket,
		Key:    aws.String(filepath.Base(filePath)),
		Body:   file,
	}); err != nil {
		return err
	}

	return nil
}

func writeFinalObjectCount(ctx context.Context, client *s3.Client, uploader *s3manager.Uploader, bucket string) error {
	finalCount, err := getObjectCount(ctx, client, bucket)
	if err != nil {
		return err
	}
	countFile, err := os.Create(FINAL_COUNT_PATH)
	if err != nil {
		return err
	}
	defer countFile.Close()
	_, err = countFile.Write([]byte(fmt.Sprintf("%d", finalCount)))
	if err != nil {
		return err
	}
	log.Printf("final count: %d", finalCount)
	return uploadToS3(ctx, uploader, bucket, FINAL_COUNT_PATH)
}

func getObjectCount(ctx context.Context, client *s3.Client, bucket string) (int, error) {
	count := 0
	listObjectParams := &s3.ListObjectsV2Input{
		Bucket: &bucket,
	}
	paginator := s3.NewListObjectsV2Paginator(client, listObjectParams, func(_ *s3.ListObjectsV2PaginatorOptions) {})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		numObjects := len(page.Contents)
		count += numObjects
	}
	return count - 1, nil
}
