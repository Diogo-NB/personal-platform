package s3

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	categoryMetadataKey = "skycrate-category"
	tierMetadataKey     = "skycrate-storage-tier"
	tierTagKey          = "storage-tier"
)

var _ out.ObjectRepository = (*Repository)(nil)

type Repository struct {
	bucket   string
	client   *awss3.Client
	uploader *transfermanager.Client
}

func New(
	bucket string,
	client *awss3.Client,
	uploader *transfermanager.Client,
) (*Repository, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("s3 repository bucket must not be empty")
	}
	if strings.TrimSpace(bucket) != bucket {
		return nil, errors.New("s3 repository bucket must not contain surrounding whitespace")
	}
	if client == nil {
		return nil, errors.New("s3 repository client must not be nil")
	}
	if uploader == nil {
		return nil, errors.New("s3 repository uploader must not be nil")
	}

	return &Repository{
		bucket:   bucket,
		client:   client,
		uploader: uploader,
	}, nil
}

func (r *Repository) Save(ctx context.Context, request out.SaveRequest) (saveErr error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save object to bucket %q: %w", r.bucket, err)
	}
	if request.SourcePath == "" {
		return fmt.Errorf("save object to bucket %q: source path must not be empty", r.bucket)
	}
	if err := request.StoredObject.Validate(); err != nil {
		return fmt.Errorf("save object to bucket %q: %w", r.bucket, err)
	}

	source, err := os.Open(request.SourcePath)
	if err != nil {
		return fmt.Errorf("open source %q: %w", request.SourcePath, err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			saveErr = errors.Join(saveErr, fmt.Errorf("close source %q: %w", request.SourcePath, err))
		}
	}()

	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect source %q: %w", request.SourcePath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("inspect source %q: not a regular file", request.SourcePath)
	}
	if info.Size() != request.StoredObject.Size {
		return fmt.Errorf(
			"inspect source %q: size changed from %d to %d bytes",
			request.SourcePath,
			request.StoredObject.Size,
			info.Size(),
		)
	}

	storageClass, err := storageClassFor(request.StoredObject.Tier)
	if err != nil {
		return fmt.Errorf("save object to bucket %q: %w", r.bucket, err)
	}
	tags := url.Values{tierTagKey: []string{request.StoredObject.Tier.String()}}.Encode()
	_, uploadErr := r.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:            aws.String(r.bucket),
		Key:               aws.String(request.StoredObject.Path),
		Body:              source,
		ChecksumAlgorithm: transfertypes.ChecksumAlgorithmSha256,
		ContentLength:     aws.Int64(request.StoredObject.Size),
		MpuObjectSize:     aws.Int64(request.StoredObject.Size),
		Metadata: map[string]string{
			categoryMetadataKey: request.StoredObject.Category,
			tierMetadataKey:     request.StoredObject.Tier.String(),
		},
		StorageClass: storageClass,
		Tagging:      aws.String(tags),
	})
	if uploadErr != nil {
		return fmt.Errorf("upload object %q to bucket %q: %w", request.StoredObject.Path, r.bucket, uploadErr)
	}

	return nil
}

func (r *Repository) FindMany(
	ctx context.Context,
) ([]object.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("find objects in bucket %q: %w", r.bucket, err)
	}

	input := &awss3.ListObjectsV2Input{Bucket: aws.String(r.bucket)}
	paginator := awss3.NewListObjectsV2Paginator(r.client, input)
	objects := []object.Object{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects in bucket %q: %w", r.bucket, err)
		}

		for _, summary := range page.Contents {
			storedObject, isManaged, err := r.rehydrate(ctx, summary)
			if err != nil {
				return nil, err
			}
			if !isManaged {
				continue
			}
			objects = append(objects, storedObject)
		}
	}

	return objects, nil
}

func (r *Repository) rehydrate(
	ctx context.Context,
	summary awss3types.Object,
) (object.Object, bool, error) {
	if summary.Key == nil || summary.Size == nil || summary.LastModified == nil {
		return object.Object{}, false, nil
	}

	head, err := r.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    summary.Key,
	})
	if err != nil {
		return object.Object{}, false, fmt.Errorf(
			"read metadata for object %q in bucket %q: %w",
			aws.ToString(summary.Key),
			r.bucket,
			err,
		)
	}

	categoryPath, hasCategory := head.Metadata[categoryMetadataKey]
	tierValue, hasTier := head.Metadata[tierMetadataKey]
	if !hasCategory || !hasTier {
		return object.Object{}, false, nil
	}
	normalizedCategory, err := category.NormalizePath(categoryPath)
	if err != nil || normalizedCategory != categoryPath {
		return object.Object{}, false, nil
	}
	tier, err := storage.Parse(tierValue)
	if err != nil {
		return object.Object{}, false, nil
	}

	timestamp := summary.LastModified.UTC()
	storedObject, err := object.Rehydrate(object.RehydrateParams{
		Path:      aws.ToString(summary.Key),
		Name:      path.Base(aws.ToString(summary.Key)),
		Category:  categoryPath,
		Size:      aws.ToInt64(summary.Size),
		Tier:      tier,
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	})
	if err != nil {
		return object.Object{}, false, nil
	}

	return storedObject, true, nil
}

func storageClassFor(tier storage.Tier) (transfertypes.StorageClass, error) {
	switch tier {
	case storage.TierDefault:
		return transfertypes.StorageClassStandard, nil
	case storage.TierArchive:
		return transfertypes.StorageClassDeepArchive, nil
	case storage.TierCold:
		return transfertypes.StorageClassGlacier, nil
	case storage.TierInstant:
		return transfertypes.StorageClassGlacierIr, nil
	default:
		return "", fmt.Errorf("map storage tier: %w", tier.Validate())
	}
}
