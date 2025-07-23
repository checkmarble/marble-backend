package infra

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
)

type BlobType int

const (
	BlobTypeS3 BlobType = iota
	BlobTypeAzure
)

type AnalyticsConfig struct {
	Type             BlobType
	Bucket           string
	ConnectionString string
}

func InitAnalyticsConfig(bucket string) (AnalyticsConfig, error) {
	u, err := url.Parse(bucket)
	if err != nil {
		return AnalyticsConfig{}, err
	}

	cfg := AnalyticsConfig{
		Bucket: bucket,
	}

	switch u.Scheme {

	case "s3":
		if err := cfg.buildS3ConnectionString(); err != nil {
			return AnalyticsConfig{}, err
		}

	default:
		return AnalyticsConfig{}, errors.New("unsupported storage for analytics")
	}

	return cfg, nil
}

func (cfg *AnalyticsConfig) buildS3ConnectionString() error {
	u, err := url.Parse(cfg.Bucket)
	if err != nil {
		return errors.Wrap(err, "could not build analytics bucket connection string")
	}

	cfg.Type = BlobTypeS3
	cfg.Bucket = fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	args := []string{
		"type s3",
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		args = append(args, []string{
			"provider config",
			fmt.Sprintf("key_id '%s'", os.Getenv("AWS_ACCESS_KEY_ID")),
			fmt.Sprintf("secret '%s'", os.Getenv("AWS_SECRET_ACCESS_KEY")),
		}...)
	} else {
		args = append(args, []string{"provider credential_chain", "chain 'env;config'"}...)
	}

	if v := u.Query().Get("endpoint"); v != "" {
		ep, err := url.Parse(v)
		if err != nil {
			return errors.Wrap(err, "could not build analytics bucket connection string")
		}

		args = append(args, fmt.Sprintf("endpoint '%s'", ep.Host))
	}
	if v := u.Query().Get("disableSSL"); v == "true" {
		args = append(args, "use_ssl 'false'")
	}
	if v := u.Query().Get("s3ForcePathStyle"); v == "true" {
		args = append(args, "url_style 'path'")
	}
	if v := u.Query().Get("region"); v != "" {
		args = append(args, fmt.Sprintf("region '%s'", v))
	}

	cfg.ConnectionString = strings.Join(args, ", ")

	return nil
}
