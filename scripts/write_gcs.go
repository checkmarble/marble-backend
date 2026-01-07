package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/gcp"
	"golang.org/x/oauth2/google"
	"golang.org/x/time/rate"
	"google.golang.org/api/compute/v1"
)

type writerStats struct {
	success uint64
	failure uint64
}

func main3() {
	var (
		bucketArg    string
		prefix       string
		rps          int
		duration     time.Duration
		concurrency  int
		bytesPerFile int
	)

	flag.StringVar(&bucketArg, "bucket", "", "Target bucket (format: gs://<bucket>)")
	flag.StringVar(&prefix, "prefix", "ratelimit-test",
		"Object key prefix to use (shared across all writes)")
	flag.IntVar(&rps, "rps", 100, "Desired writes per second")
	flag.DurationVar(&duration, "duration", time.Minute, "How long to run the test")
	flag.IntVar(&concurrency, "concurrency", 32, "Number of concurrent workers")
	flag.IntVar(&bytesPerFile, "bytes", 0, "Bytes to write per object (0 = empty file)")
	flag.Parse()

	if bucketArg == "" {
		log.Fatal("-bucket is required (e.g. gs://my-bucket)")
	}

	ctx := context.Background()

	bu, err := url.Parse(bucketArg)
	if err != nil {
		log.Fatalf("invalid bucket URL: %v", err)
	}
	if bu.Scheme != "gs" || bu.Host == "" {
		log.Fatalf("bucket must be a gs:// URL, got %q", bucketArg)
	}

	creds, err := google.FindDefaultCredentials(ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatalf("failed to find default credentials: %v", err)
	}
	client, err := gcp.NewHTTPClient(gcp.DefaultTransport(), gcp.CredentialsTokenSource(creds))
	if err != nil {
		log.Fatalf("failed to create HTTP client: %v", err)
	}

	bucket, err := gcsblob.OpenBucket(ctx, client, bu.Host, nil)
	if err != nil {
		log.Fatalf("failed to open bucket: %v", err)
	}
	defer bucket.Close()

	trimmedPrefix := strings.Trim(prefix, "/")
	if trimmedPrefix != "" {
		trimmedPrefix += "/"
	}

	limiter := rate.NewLimiter(rate.Limit(rps), rps)
	var stats writerStats

	log.Printf("Starting GCS write rate test: bucket=%s prefix=%s rps=%d duration=%s concurrency=%d bytes=%d",
		bu.Host, trimmedPrefix, rps, duration.String(), concurrency, bytesPerFile)

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for w := 0; w < concurrency; w++ {
		go func(workerId int) {
			defer wg.Done()
			buf := make([]byte, bytesPerFile)
			if bytesPerFile > 0 {
				// Fill with deterministic but varied content
				rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerId))).Read(buf)
			}
			for {
				if err := limiter.Wait(ctx); err != nil {
					return
				}

				key := fmt.Sprintf("%s%d-%d-%06d", trimmedPrefix, time.Now().Unix(),
					time.Now().UnixNano()%1e9, rand.Intn(1_000_000))

				wr, err := bucket.NewWriter(ctx, key, &blob.WriterOptions{
					ContentType: "application/octet-stream",
				})
				if err != nil {
					atomic.AddUint64(&stats.failure, 1)
					continue
				}
				if bytesPerFile > 0 {
					if _, err := wr.Write(buf); err != nil {
						_ = wr.Close()
						atomic.AddUint64(&stats.failure, 1)
						continue
					}
				}
				if err := wr.Close(); err != nil {
					atomic.AddUint64(&stats.failure, 1)
					continue
				}
				atomic.AddUint64(&stats.success, 1)
			}
		}(w)
	}

	// Progress reporter
	progCtx, progCancel := context.WithCancel(context.Background())
	var lastSuccess, lastFailure uint64
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-progCtx.Done():
				return
			case <-t.C:
				curS := atomic.LoadUint64(&stats.success)
				curF := atomic.LoadUint64(&stats.failure)
				log.Printf("progress: success=%d (+%d/s) failure=%d (+%d/s)", curS,
					curS-lastSuccess, curF, curF-lastFailure)
				lastSuccess, lastFailure = curS, curF
			}
		}
	}()

	wg.Wait()
	progCancel()

	log.Printf("Completed. success=%d failure=%d", atomic.LoadUint64(&stats.success), atomic.LoadUint64(&stats.failure))
}
