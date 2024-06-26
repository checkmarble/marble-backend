package models

type GlobalConfiguration struct {
	FakeAwsS3Repository              bool
	FakeGcsRepository                bool
	GcsIngestionBucket               string
	GcsCaseManagerBucket             string
	GcsTransferCheckEnrichmentBucket string
	JwtSigningKey                    string
	MarbleAppHost                    string
	MarbleBackofficeHost             string
	SegmentWriteKey                  string
	TokenLifetimeMinute              int
}
