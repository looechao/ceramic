package draftstore

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
)

var ErrImageSize = errors.New("image exceeds 16 MiB")
var ErrImageFormat = errors.New("invalid image or image exceeds 24 megapixels")

func ReadImage(path string) (Image, error) {
	var a Image
	f, err := os.Open(path)
	if err != nil {
		return a, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return a, err
	}
	if len(data) > 16<<20 {
		return a, ErrImageSize
	}
	err = ValidateImage(data)
	if err != nil {
		return a, err
	}
	a = Image{Name: filepath.Base(path), Data: data, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}
	return a, nil
}

func ValidateImage(data []byte) error {
	if len(data) > 16<<20 {
		return ErrImageSize
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 24000000 {
		return ErrImageFormat
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return ErrImageFormat
	}
	return nil
}
