package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

// maxJobCount, _ = strconv.ParseInt(config.MaxJobCount, 10, 64)// maxJobCounter = strconv.ParseInt(config.MaxJobCount, 10, 64)
var currentJobCounter int64 = 0

func changeCounter(delta int64) int64 {
	return atomic.AddInt64(&currentJobCounter, delta)
}

func getCounter() int64 {
	return atomic.LoadInt64(&currentJobCounter)
}

func convert(c *fiber.Ctx) error {

	//basic vars
	var reqURI, _ = url.QueryUnescape(c.Path()) // /mypic/123.jpg
	var rawImageAbs string
	if proxyMode {
		rawImageAbs = config.ImgPath + reqURI
	} else {
		rawImageAbs = path.Join(config.ImgPath, reqURI) // /home/xxx/mypic/123.jpg
	}
	var imgFilename = path.Base(reqURI) // pure filename, 123.jpg
	var finalFile string                // We'll only need one c.sendFile()
	var ua = c.Get("User-Agent")
	var accept = c.Get("accept")
	log.Debugf("Incoming connection from %s@%s with %s", ua, c.IP(), imgFilename)

	needOrigin := goOrigin(accept, ua)
	if needOrigin {
		log.Debugf("A Safari/IE/whatever user has arrived...%s", ua)
		c.Set("ETag", genEtag(rawImageAbs))
		if proxyMode {
			localRemoteTmpPath := remoteRaw + reqURI
			_ = fetchRemoteImage(localRemoteTmpPath, rawImageAbs)
			return c.SendFile(localRemoteTmpPath)
		} else {
			return c.SendFile(rawImageAbs)
		}
	}

	// check ext
	var allowed = false
	for _, ext := range config.AllowedTypes {
		haystack := strings.ToLower(imgFilename)
		needle := strings.ToLower("." + ext)
		if strings.HasSuffix(haystack, needle) {
			allowed = true
			break
		} else {
			allowed = false
		}
	}

	if !allowed {
		msg := "File extension not allowed! " + imgFilename
		log.Warn(msg)
		if imageExists(rawImageAbs) {
			c.Set("ETag", genEtag(rawImageAbs))
			return c.SendFile(rawImageAbs)
		} else {
			c.Status(http.StatusBadRequest)
			_ = c.Send([]byte(msg))
			return nil
		}
	}

	if proxyMode {
		return proxyHandler(c, reqURI)
	}

	// Check the original image for existence,
	if !imageExists(rawImageAbs) {
		msg := "image not found"
		_ = c.Send([]byte(msg))
		log.Warn(msg)
		_ = c.SendStatus(404)
		return errors.New(msg)
	}

	_, webpAbsPath := genWebpAbs(rawImageAbs, config.ExhaustPath, imgFilename, reqURI)

	if imageExists(webpAbsPath) {
		finalFile = webpAbsPath
	} else {
		// we don't have abc.jpg.png1582558990.webp
		// delete the old pic and convert a new one.
		// /home/webp_server/exhaust/path/to/tsuki.jpg.1582558990.webp
		destHalfFile := path.Clean(path.Join(webpAbsPath, path.Dir(reqURI), imgFilename))
		matches, err := filepath.Glob(destHalfFile + "*")
		if err != nil {
			log.Error(err.Error())
		} else {
			// /home/webp_server/exhaust/path/to/tsuki.jpg.1582558100.webp <- older ones will be removed
			// /home/webp_server/exhaust/path/to/tsuki.jpg.1582558990.webp <- keep the latest one
			for _, p := range matches {
				if strings.Compare(destHalfFile, p) != 0 {
					_ = os.Remove(p)
				}
			}
		}

		// Get global counter
		for getCounter() >= maxJobCount {
			log.Debugf("Max job of %d met, not converting and waiting for other request to complete.", maxJobCount)
			time.Sleep(50 * time.Millisecond)
		}
		changeCounter(1)
		//for webp, we need to create dir first
		err = os.MkdirAll(path.Dir(webpAbsPath), 0755)
		q, _ := strconv.ParseFloat(config.Quality, 32)
		err = webpEncoder(rawImageAbs, webpAbsPath, float32(q), true, nil)
		changeCounter(-1)

		if err != nil {
			log.Error(err)
			_ = c.SendStatus(400)
			_ = c.Send([]byte("Bad file. " + err.Error()))
			return err
		}
		finalFile = webpAbsPath
	}
	etag := genEtag(finalFile)
	c.Set("ETag", etag)
	c.Set("X-Compression-Rate", getCompressionRate(rawImageAbs, webpAbsPath))
	finalFile = chooseLocalSmallerFile(rawImageAbs, webpAbsPath)
	// defer os.Remove(webpAbsPath)

	return c.SendFile(finalFile)

}

func proxyHandler(c *fiber.Ctx, reqURI string) error {
	// https://test.webp.sh/node.png
	realRemoteAddr := config.ImgPath + reqURI
	// Ping Remote for status code and etag info
	log.Infof("Remote Addr is %s fetching", realRemoteAddr)
	statusCode, etagValue, remoteLength := getRemoteImageInfo(realRemoteAddr)
	if statusCode == 200 {
		// Check local path: /node.png-etag-<etagValue>
		localEtagWebPPath := config.ExhaustPath + reqURI + "-etag-" + etagValue
		if imageExists(localEtagWebPPath) {
			chooseProxy(remoteLength, localEtagWebPPath)
			return c.SendFile(localEtagWebPPath)
		} else {
			// Temporary store of remote file.
			cleanProxyCache(config.ExhaustPath + reqURI + "*")
			localRawImagePath := remoteRaw + reqURI
			_ = fetchRemoteImage(localRawImagePath, realRemoteAddr)
			q, _ := strconv.ParseFloat(config.Quality, 32)
			_ = os.MkdirAll(path.Dir(localEtagWebPPath), 0755)
			err := webpEncoder(localRawImagePath, localEtagWebPPath, float32(q), true, nil)
			if err != nil {
				log.Warning(err)
			}
			chooseProxy(remoteLength, localEtagWebPPath)
			return c.SendFile(localEtagWebPPath)
		}
	} else {
		msg := fmt.Sprintf("Remote returned %d status code!", statusCode)
		_ = c.Send([]byte(msg))
		log.Warn(msg)
		_ = c.SendStatus(statusCode)
		cleanProxyCache(config.ExhaustPath + reqURI + "*")
		return errors.New(msg)
	}
}
