package providers

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/bitrise-io/api-utils/httpresponse"
	"github.com/pkg/errors"
)

// AWSInterface ...
type AWSInterface interface {
	GeneratePresignedGETURL(key string, expiresIn time.Duration) (string, error)
	GeneratePresignedPUTURL(key string, expiresIn time.Duration, fileSize int64) (string, error)
	GetObject(key string) (string, error)
	GetConfig() AWSConfig
	PutObject(key string, objectBytes []byte) error
	MoveObject(from string, to string) error
	DeleteObject(path string) error
}

// AWSConfig ...
type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

// AWS ...
type AWS struct {
	Config AWSConfig
}

// GetConfig ...
func (p *AWS) GetConfig() AWSConfig {
	return p.Config
}

func (p *AWS) createS3Client() (svc *s3.S3, err error) {
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			p.Config.AccessKeyID,
			p.Config.SecretAccessKey,
			""),
		Region: aws.String(p.Config.Region),
	})
	if err != nil {
		return nil, errors.Wrap(err, "Session creation failed")
	}

	svc = s3.New(sess)
	return
}

// GeneratePresignedGETURL ...
func (p *AWS) GeneratePresignedGETURL(key string, expiresIn time.Duration) (string, error) {
	svc, err := p.createS3Client()
	if err != nil {
		return "", errors.WithStack(err)
	}

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(p.Config.Bucket),
		Key:    aws.String(key),
	})
	presignedURL, err := req.Presign(expiresIn)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return presignedURL, nil
}

// GeneratePresignedPUTURL ...
func (p *AWS) GeneratePresignedPUTURL(key string, expiresIn time.Duration, fileSize int64) (string, error) {
	svc, err := p.createS3Client()
	if err != nil {
		return "", errors.WithStack(err)
	}

	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket:        aws.String(p.Config.Bucket),
		Key:           aws.String(key),
		ContentLength: aws.Int64(fileSize),
	})
	presignedURL, err := req.Presign(expiresIn)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return presignedURL, nil
}

// GetObject ...
func (p *AWS) GetObject(key string) (string, error) {
	presignedURL, err := p.GeneratePresignedGETURL(key, 10*time.Minute)
	if err != nil {
		return "", errors.WithStack(err)
	}

	resp, err := http.Get(presignedURL)
	if err != nil {
		return "", errors.WithStack(err)
	}

	defer httpresponse.BodyCloseWithErrorLog(resp)
	if resp.StatusCode == 200 {
		return "", errors.New(resp.Status)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return string(bodyBytes), nil
}

// PutObject ...
func (p *AWS) PutObject(key string, objectBytes []byte) error {
	svc, err := p.createS3Client()
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(p.Config.Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(objectBytes),
	})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// MoveObject ...
func (p *AWS) MoveObject(from string, to string) error {
	svc, err := p.createS3Client()
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = svc.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(p.Config.Bucket),
		Key:        aws.String(to),
		CopySource: aws.String(fmt.Sprintf("%s/%s", p.Config.Bucket, from)),
		ACL:        aws.String("public-read"),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	err = p.DeleteObject(from)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// DeleteObject ...
func (p *AWS) DeleteObject(path string) error {
	svc, err := p.createS3Client()
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(p.Config.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
