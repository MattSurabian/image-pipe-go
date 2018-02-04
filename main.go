package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/zsais/go-gin-prometheus"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"io"
	"github.com/minio/minio-go"
	"github.com/spf13/cobra/cobra/cmd"
)

const version = "dev"

func main() {
	addr := os.Getenv("IMAGE_PIPE_HTTP_ADDR")
	if addr == "" {
		addr = ":3000"
	}

	debug := os.Getenv("IMAGE_PIPE_DEBUG")
	if debug == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		log.Fatal("AWS_ACCESS_KEY_ID must be set.")
	}

	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		log.Fatal("AWS_SECRET_ACCESS_KEY must be set.")
	}

	s := &http.Server{Addr: addr, Handler: router()}

	go func() {
		log.Printf("image-pipe service listening on %s", addr)
		if err := s.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting...")

	s.Shutdown(context.Background())
}

func router() *gin.Engine {
	router := gin.New()

	metrics := ginprometheus.NewPrometheus("gin")
	metrics.Use(router)

	v1 := router.Group("/v1")
	{
		v1.POST("/", mainEndpoint)
	}
	router.GET("/health", healthEndpoint)
	router.GET("/version", versionEndpoint)
	router.Use(defaultEndpoint)
	return router
}

type resizeRequest struct {
	URI    string `json:"uri" binding:"required"`
	Key    string `json:"key" binding:"required"`
	Bucket string `json:"bucket" binding:"required"`
	Width  string `json:"width" binding:"required"`
}

func mainEndpoint(c *gin.Context) {
	var image resizeRequest
	c.BindJSON(&image)

	// Fetch remote image via http
	response, err := http.Get(image.URI)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	// Setup imagemagick convert call and get in/out pipe references
	resizer := Resizer(image.Width)
	resizerOut, _ := resizer.StdoutPipe()
	resizerIn, _ := resizer.StdinPipe()

	// Get an S3 client to use for uploading
	client, err := GetS3Client(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if err != nil {
		log.Fatal("Problem accessing AWS S3: ", err)
	}

	// Our resizer command provides a wait, but uploading to S3 does not. Wait group to the rescue.
	wg := sync.WaitGroup{}
	wg.Add(1)

	// Routine to take from the http request and stream into the resizer
	go func() {
		defer resizerIn.Close()
		io.Copy(resizerIn, response.Body)
		resizer.Start()
	}()

	// Routine to take from the resizer and send to S3
	go func() {
		defer resizerOut.Close()
		client.PutObject(image.Bucket, image.Key, resizerOut, -1, minio.PutObjectOptions{})
		wg.Done()
	}()

	// error catching on the wait would be problematic because of our go routines, if err info is needed
	// better to report resizer.StderrPipe
	resizer.Wait()
	wg.Wait()

	c.JSON(http.StatusOK, image)
}

func healthEndpoint(c *gin.Context) {
	s := http.StatusOK
	c.String(s, http.StatusText(s))
}

func versionEndpoint(c *gin.Context) {
	c.String(http.StatusOK, version)
}

func defaultEndpoint(c *gin.Context) {
	s := http.StatusNotImplemented
	c.String(s, http.StatusText(s))
}
