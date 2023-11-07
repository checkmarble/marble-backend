package models

type GlobalConfiguration struct {
	TokenLifetimeMinute int
	FakeAwsS3Repository bool
	FakeGcsRepository   bool
	GcsIngestionBucket  string
	SegmentWriteKey     string
}
