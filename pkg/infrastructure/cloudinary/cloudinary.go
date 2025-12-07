package cloudinary

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type CloudinaryService interface {
	UploadFile(ctx context.Context, fileHeader *multipart.FileHeader, folder string) (string, error)
	DeleteFile(ctx context.Context, publicID string) error
	GetImageURL(publicID string, transformations map[string]interface{}) string
}

type cloudinaryService struct {
	client    *cloudinary.Cloudinary
	cloudName string
	logger    *logrus.Logger
	tracer    trace.TracerProvider
}

func NewCloudinaryService(client *cloudinary.Cloudinary, cloudName string, logger *logrus.Logger, tracer trace.TracerProvider) CloudinaryService {
	return &cloudinaryService{
		client:    client,
		cloudName: cloudName,
		logger:    logger,
		tracer:    tracer,
	}
}

func (s *cloudinaryService) trace(ctx context.Context, name string) (context.Context, trace.Span) {
	tracer := s.tracer.Tracer("cloudinary.service")
	return tracer.Start(ctx, name)
}

func (s *cloudinaryService) UploadFile(ctx context.Context, fileHeader *multipart.FileHeader, folder string) (string, error) {
	ctx, span := s.trace(ctx, "cloudinary.upload-file")
	defer span.End()

	// Set span attributes
	span.SetAttributes(
		attribute.String("cloudinary.filename", fileHeader.Filename),
		attribute.String("cloudinary.folder", folder),
		attribute.Int64("cloudinary.size", fileHeader.Size),
	)

	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		s.logger.Errorf("Failed to open uploaded file: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}
	defer file.Close()

	// Generate public ID from filename (without extension)
	ext := filepath.Ext(fileHeader.Filename)
	publicID := strings.TrimSuffix(fileHeader.Filename, ext)

	// Prepare upload options
	overwrite := true
	invalidate := true
	uploadOptions := &uploader.UploadParams{
		PublicID:     publicID,
		Folder:       folder,
		ResourceType: "auto", // Auto-detect resource type
		Overwrite:    &overwrite,
		Invalidate:   &invalidate,
	}

	// Upload to Cloudinary
	result, err := s.client.Upload.Upload(ctx, file, *uploadOptions)
	if err != nil {
		s.logger.Errorf("Failed to upload file to Cloudinary: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	// Set success attributes
	span.SetAttributes(
		attribute.String("cloudinary.public_id", result.PublicID),
		attribute.String("cloudinary.secure_url", result.SecureURL),
		attribute.String("cloudinary.resource_type", result.ResourceType),
	)
	span.SetStatus(codes.Ok, "File uploaded successfully")

	s.logger.Infof("Successfully uploaded file to Cloudinary: %s", result.PublicID)
	return result.SecureURL, nil
}

func (s *cloudinaryService) DeleteFile(ctx context.Context, publicID string) error {
	ctx, span := s.trace(ctx, "cloudinary.delete-file")
	defer span.End()

	// Extract public ID from URL if it's a full URL
	originalPublicID := publicID
	if strings.Contains(publicID, "cloudinary.com") {
		// Extract public ID from Cloudinary URL
		parts := strings.Split(publicID, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Remove file extension
			ext := filepath.Ext(lastPart)
			publicID = strings.TrimSuffix(lastPart, ext)
		}
	}

	// Set span attributes
	span.SetAttributes(
		attribute.String("cloudinary.original_public_id", originalPublicID),
		attribute.String("cloudinary.public_id", publicID),
	)

	// Delete from Cloudinary
	result, err := s.client.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: publicID,
	})
	if err != nil {
		s.logger.Errorf("Failed to delete file from Cloudinary: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Set success attributes
	span.SetAttributes(
		attribute.String("cloudinary.delete_result", result.Result),
	)
	span.SetStatus(codes.Ok, "File deleted successfully")

	s.logger.Infof("Successfully deleted file from Cloudinary: %s, Result: %s", publicID, result.Result)
	return nil
}

func (s *cloudinaryService) GetImageURL(publicID string, transformations map[string]interface{}) string {
	_, span := s.trace(context.Background(), "cloudinary.get-image-url")
	defer span.End()

	// Set span attributes
	span.SetAttributes(
		attribute.String("cloudinary.public_id", publicID),
		attribute.String("cloudinary.cloud_name", s.cloudName),
	)
	if len(transformations) > 0 {
		span.SetAttributes(
			attribute.String("cloudinary.transformations", fmt.Sprintf("%+v", transformations)),
		)
	}

	// Generate Cloudinary URL directly using string formatting
	url := fmt.Sprintf("https://res.cloudinary.com/%s/image/upload/%s", s.cloudName, publicID)

	// Set success attributes
	span.SetAttributes(
		attribute.String("cloudinary.generated_url", url),
	)
	span.SetStatus(codes.Ok, "Image URL generated successfully")

	return url
}
