package worker_jobs

import (
	"context"
	"image"
	"io"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/disintegration/imaging"
	"github.com/gabriel-vasile/mimetype"
	"github.com/riverqueue/river"
)

const THUMBNAIL_WIDTH = 300

type GenerateThumbnailWorker struct {
	river.WorkerDefaults[models.GenerateThumbnailArgs]

	blobRepository repositories.BlobRepository
}

func NewGenerateThumbnailWorker(
	blobRepository repositories.BlobRepository,
) *GenerateThumbnailWorker {
	return &GenerateThumbnailWorker{
		blobRepository: blobRepository,
	}
}

func (w GenerateThumbnailWorker) Work(ctx context.Context, job *river.Job[models.GenerateThumbnailArgs]) error {
	b, err := w.blobRepository.GetBlob(ctx, job.Args.Bucket, job.Args.Key)
	if err != nil {
		return err
	}
	defer b.ReadCloser.Close()

	mime, err := mimetype.DetectReader(b.ReadCloser)
	if err != nil {
		return err
	}

	if _, err := b.ReadCloser.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var img *image.NRGBA

	switch {
	case mime.Is("application/pdf"):
		// TODO: find a new way to generate PDF thumbnail
		return nil
	case strings.HasPrefix(mime.String(), "image/"):
		img, err = w.createImageThumbnail(ctx, b.ReadCloser)
	default:
		return nil
	}

	if err != nil {
		return err
	}

	wr, err := w.blobRepository.OpenStream(ctx, job.Args.Bucket, models.ThumbnailFileName(job.Args.Key), "")
	if err != nil {
		return err
	}
	defer wr.Close()

	return imaging.Encode(wr, img, imaging.JPEG)
}

func (w GenerateThumbnailWorker) createImageThumbnail(_ context.Context, r io.Reader) (*image.NRGBA, error) {
	src, err := imaging.Decode(r)
	if err != nil {
		return nil, err
	}

	return imaging.Resize(src, THUMBNAIL_WIDTH, 0, imaging.Lanczos), nil
}
