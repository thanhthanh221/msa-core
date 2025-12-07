package minio

import (
	"context"
	"mime/multipart"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type MinioService interface {
	UploadFile(ctx context.Context, file *multipart.FileHeader, folder string) (string, error)
	DownloadFile(ctx context.Context, fileID string, folder string) (string, error)
	DeleteFile(ctx context.Context, fileID string, folder string) error
}

type minioService struct {
	minioClient  *minio.Client
	bucketName   string
	bucketRegion string
	logger       *logrus.Logger
	tracer       trace.TracerProvider
}

func NewMinioService(minioClient *minio.Client, bucketName string, bucketRegion string, logger *logrus.Logger, tracer trace.TracerProvider) MinioService {
	return &minioService{
		minioClient:  minioClient,
		bucketName:   bucketName,
		bucketRegion: bucketRegion,
		logger:       logger,
		tracer:       tracer,
	}
}

func (s *minioService) trace(ctx context.Context, name string) (context.Context, trace.Span) {
	tracer := s.tracer.Tracer("minio.service")
	return tracer.Start(ctx, name)
}

func (s *minioService) UploadFile(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	ctx, span := s.trace(ctx, "minio.upload-file")
	defer span.End()

	bucket := s.bucketName

	// Set span attributes
	span.SetAttributes(
		attribute.String("minio.filename", file.Filename),
		attribute.String("minio.folder", folder),
		attribute.String("minio.bucket", bucket),
		attribute.Int64("minio.size", file.Size),
	)

	if err := s.ensureBucket(ctx, bucket); err != nil {
		s.logger.Errorf("Failed to ensure bucket: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	src, err := file.Open()
	if err != nil {
		s.logger.Errorf("Failed to open file: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}
	defer src.Close()

	objectName := folder + "/" + time.Now().Format("20060102150405") + "-" + file.Filename
	contentType := file.Header.Get("Content-Type")

	_, err = s.minioClient.PutObject(ctx, bucket, objectName, src, file.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		s.logger.Errorf("Failed to upload file to MinIO: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	// Set success attributes
	span.SetAttributes(
		attribute.String("minio.object_name", objectName),
		attribute.String("minio.content_type", contentType),
	)
	span.SetStatus(codes.Ok, "File uploaded successfully")

	s.logger.Infof("Successfully uploaded file to MinIO: %s", objectName)
	return objectName, nil
}

func (s *minioService) DownloadFile(ctx context.Context, fileID string, folder string) (string, error) {
	bucket := s.bucketName

	url, err := s.minioClient.PresignedGetObject(ctx, bucket, folder+"/"+fileID, time.Hour, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

func (s *minioService) DeleteFile(ctx context.Context, fileID string, folder string) error {
	ctx, span := s.trace(ctx, "minio.delete-file")
	defer span.End()

	bucket := s.bucketName
	objectName := folder + "/" + fileID

	// Set span attributes
	span.SetAttributes(
		attribute.String("minio.file_id", fileID),
		attribute.String("minio.folder", folder),
		attribute.String("minio.bucket", bucket),
		attribute.String("minio.object_name", objectName),
	)

	err := s.minioClient.RemoveObject(ctx, bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		s.logger.Errorf("Failed to delete file from MinIO: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "File deleted successfully")
	s.logger.Infof("Successfully deleted file from MinIO: %s", objectName)
	return nil
}

func (s *minioService) ensureBucket(ctx context.Context, bucket string) error {
	exists, err := s.minioClient.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: s.bucketRegion}); err != nil {
			return err
		}
	}
	return nil
}
