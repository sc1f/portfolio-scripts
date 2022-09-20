package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/disintegration/gift"
)

const (
	LONG_EDGE             = 2560
	BUCKET                = "juntan-portfolio-images"
	S3_REGION             = "us-east-1"
	PROCESSED_FOLDER_NAME = "processed"
)

var jpegRegex, _ = regexp.Compile("[Jj]{1}[Pp]{1}[Ee]?[Gg]{1}")
var resizeFilter *gift.GIFT

type imageForProcessing struct {
	imagePath  string
	savePath   string
	bucket     string
	s3Uploader *s3manager.Uploader
}

func cleanup(dir string) error {
	return os.RemoveAll(dir)
}

func processAndUploadImage(image *imageForProcessing) {
	ctx := context.Background()
	log.Printf("processing image %s", image.imagePath)
	err := processImage(ctx, image.imagePath, image.savePath)
	if err != nil {
		log.Printf("ERROR: could not processImage %s, saved to %s, err: %s", image.imagePath, image.savePath, err)
		return
	}
	log.Printf("uploading image %s", image.savePath)
	err = uploadToS3(ctx, image.s3Uploader, image.bucket, image.savePath)
	if err != nil {
		log.Printf("ERROR: could not uploadToS3 %s to bucket %s, err: %s", image.savePath, image.bucket, err)
		return
	}
	log.Printf("uploaded image %s", image.savePath)
}

func main() {
	d := flag.String("dir", "", "directory containing unprocessed images")
	b := flag.String("bucket", BUCKET, "S3 bucket for the photos to be uploaded into")
	s := flag.Int("size", LONG_EDGE, "the maximum long edge size of each image")
	flag.Parse()

	if d == nil || *d == "" {
		panic("cannot process images without a directory")
	}

	directory := *d
	bucket := *b
	size := *s
	dir, err := filepath.Abs(directory)
	if err != nil {
		log.Panicf("could not get directory, err: %s", err)
	}

	ctx := context.Background()
	s3Client, err := connectToS3(ctx)
	if err != nil {
		log.Panicf("could not connect to s3, err: %s", err)
	}
	initialCount, err := getObjectCount(ctx, s3Client, bucket)
	if err != nil {
		log.Panicf("could not get initial count, err: %s", err)
	}

	images, err := listImages(ctx, dir)
	if err != nil {
		log.Panicf("could not list images, err: %s", err)
	}

	// make a new directory to hold the processed images
	processedDir := filepath.Join(dir, PROCESSED_FOLDER_NAME)
	err = os.MkdirAll(processedDir, os.ModePerm)
	if err != nil {
		log.Panicf("could not create dir %s in path %s, error: %s", PROCESSED_FOLDER_NAME, dir, err)
	}

	resizeFilter = gift.New(gift.Resize(size, 0, gift.LanczosResampling))

	s3Uploader, err := s3Uploader(S3_REGION)
	if err != nil {
		log.Panicf("could not get s3 uploader, err: %s", err)
	}

	total := len(images)
	log.Printf("processing %d images", total)
	var group sync.WaitGroup
	for idx, image := range images {
		idx := idx
		image := image
		newIdx := initialCount + idx
		savePath := filepath.Join(processedDir, fmt.Sprintf("%d%s", newIdx, filepath.Ext(image)))
		group.Add(1)

		go func() {
			defer group.Done()
			processAndUploadImage(&imageForProcessing{
				imagePath:  image,
				savePath:   savePath,
				bucket:     bucket,
				s3Uploader: s3Uploader,
			})
		}()
	}
	group.Wait()

	log.Printf("processed and uploaded %d images", total)

	err = writeFinalObjectCount(ctx, s3Client, s3Uploader, bucket)
	if err != nil {
		log.Panicf("could not write final object count to s3, err: %s", err)
	}

	log.Printf("removing directory %s", processedDir)
	err = cleanup(processedDir)
	if err != nil {
		log.Panicf("could not remove directory %s, err: %s", processedDir, err)
	}
}
