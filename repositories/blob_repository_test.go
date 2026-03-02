package repositories

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func bucketHostRepo(t *testing.T, endpoint, region string) *blobRepository {
	t.Helper()
	t.Setenv("AWS_ENDPOINT_URL", endpoint)
	t.Setenv("AWS_REGION", region)

	return &blobRepository{}
}

func TestBucketHost_baseUrlOnly(t *testing.T) {
	r := bucketHostRepo(t, "", "")

	assert.Equal(t, []string{"https://s3.amazonaws.com"}, r.ExtractHost("s3://bucket"))
	assert.Equal(t, []string{"https://storage.googleapis.com"}, r.ExtractHost("gs://bucket"))
	assert.Equal(t, []string{"https://*.blob.core.windows.net"}, r.ExtractHost("azblob://bucket"))
}

func TestBucketHost_s3_overrideUrlOnly(t *testing.T) {
	r := bucketHostRepo(t, "", "")

	assert.Equal(t, []string{"local.lan:3000"}, r.ExtractHost("s3://bucket?endpoint=local.lan:3000"))
	assert.Equal(t, []string{"https://s3.amazonaws.com", "https://s3.us-west-1.amazonaws.com"}, r.ExtractHost("s3://bucket?region=us-west-1"))
}

func TestBucketHost_s3_overrideEnvvar(t *testing.T) {
	r := bucketHostRepo(t, "http://local.lan:3000", "")

	assert.Equal(t, []string{"http://local.lan:3000"}, r.ExtractHost("s3://bucket"))

	r = bucketHostRepo(t, "", "eu-central-1")

	assert.Equal(t, []string{"https://s3.amazonaws.com", "https://s3.eu-central-1.amazonaws.com"}, r.ExtractHost("s3://bucket"))
}
