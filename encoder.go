package main

import (
	"bytes"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/chai2010/webp"
	"golang.org/x/image/bmp"
)

func webpEncoder(rawImageAbs, webpAbsPath string, quality float32, Log bool, c chan int) (err error) {
	// if convert fails, return error; success nil

	// Create a lock file to prevent multiple conversion on one file at once
	lockFile := webpAbsPath + ".lock"

	for fileExists(lockFile) {
		log.Debugf("lock of %s met, not converting and waiting for other request to complete.", lockFile)
		time.Sleep(50 * time.Millisecond)
	}
	os.Create(lockFile)

	log.Debugf("target: %s with quality of %f", path.Base(rawImageAbs), quality)
	var buf bytes.Buffer
	var img image.Image

	data, err := ioutil.ReadFile(rawImageAbs)
	if err != nil {
		chanErr(c)
		return
	}

	contentType := getFileContentType(data[:512])
	if strings.Contains(contentType, "jpeg") {
		img, _ = jpeg.Decode(bytes.NewReader(data))
	} else if strings.Contains(contentType, "png") {
		img, _ = png.Decode(bytes.NewReader(data))
	} else if strings.Contains(contentType, "bmp") {
		img, _ = bmp.Decode(bytes.NewReader(data))
	} else if strings.Contains(contentType, "gif") {
		// TODO: need to support animated webp
		log.Warn("Gif support is not perfect!")
		img, _ = gif.Decode(bytes.NewReader(data))
	}

	if img == nil {
		msg := "image file " + path.Base(rawImageAbs) + " is corrupted or not supported"
		log.Debug(msg)
		err = errors.New(msg)
		chanErr(c)
		return
	}

	if err = webp.Encode(&buf, img, &webp.Options{Lossless: false, Quality: quality}); err != nil {
		log.Error(err)
		chanErr(c)
		return
	}
	if err = ioutil.WriteFile(webpAbsPath, buf.Bytes(), 0644); err != nil {
		log.Error(err)
		chanErr(c)
		return
	}

	if Log {
		log.Info("Save to " + webpAbsPath + " ok!\n")
	}

	chanErr(c)

	// Delete lockfile after conversion
	defer os.Remove(lockFile)

	return nil
}
