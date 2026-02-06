package worker_jobs

import (
	"bytes"
	"context"
	"image"
	"io"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
	"github.com/disintegration/imaging"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gen2brain/go-fitz"
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
		img, err = w.createPdfThumbnail(b.ReadCloser)
	case strings.HasPrefix(mime.String(), "image/"):
		img, err = w.createImageThumbnail(b.ReadCloser)
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

func (w GenerateThumbnailWorker) createPdfThumbnail(r io.Reader) (*image.NRGBA, error) {
	doc, err := fitz.NewFromReader(r)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	if doc.NumPage() == 0 {
		return nil, errors.New("PDF document contains zero pages")
	}

	img, err := doc.Image(0)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	if err := imaging.Encode(&buf, img, imaging.JPEG); err != nil {
		return nil, err
	}

	return w.createImageThumbnail(&buf)
}

func (w GenerateThumbnailWorker) createImageThumbnail(r io.Reader) (*image.NRGBA, error) {
	src, err := imaging.Decode(r)
	if err != nil {
		return nil, err
	}

	return imaging.Resize(src, THUMBNAIL_WIDTH, 0, imaging.Lanczos), nil
}
