package main

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
)

// listImages gets all JPEGs at the top level of `dir` (non-recursive)
func listImages(ctx context.Context, dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var images []string
	for _, file := range files {
		name := file.Name()
		if file.IsDir() || !jpegRegex.MatchString(name) {
			continue
		}
		images = append(images, filepath.Join(dir, name))
	}
	return images, nil
}

func processImage(ctx context.Context, imagePath string, savePath string) error {
	if resizeFilter == nil {
		return fmt.Errorf("cannot resize with a nil filter")
	}

	// read the image
	f, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("could not read image %s, err: %s", imagePath, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("could not decode image %s, err %s", imagePath, err)
	}

	// create a destination image
	resized := image.NewRGBA(resizeFilter.Bounds(img.Bounds()))

	// resize
	resizeFilter.Draw(resized, img)

	// save
	resizedFile, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("could not create image, err: %s", err)
	}
	defer resizedFile.Close()

	if err = jpeg.Encode(resizedFile, resized, nil); err != nil {
		return fmt.Errorf("could not write resized file %s, err: %s", savePath, err)
	}

	return nil
}
