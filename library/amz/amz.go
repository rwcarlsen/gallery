
package amz

import (
	"log"
	"errors"
	"path/filepath"
	"launchpad.net/goamz/s3"
	"launchpad.net/goamz/aws"
	"net/http"
)

type S3Backend struct {
	s3link *s3.S3
}

func New(auth aws.Auth, region aws.Region) *S3Backend {
	return &S3Backend{
		s3link: s3.New(auth, region),
	}
}

func (lb *S3Backend) Put(path, name string, data []byte) error {
	items := filepath.SplitList(path)
	if len(items) == 0 {
		return errors.New("amz: path is too short")
	}
	bucket := lb.s3link.Bucket(items[0])
	bpath := filepath.Join(items[1:]...)

	// make sure bucket exists - create if needed
	err := bucket.PutBucket(s3.Private)
	if err != nil {
		log.Println(err.Error())
	}

	contType := http.DetectContentType(data)
	return bucket.Put(bpath, data, contType, s3.Private)
}

func (lb *S3Backend) Exists(path, name string) bool {
	return true
}

func (lb *S3Backend) Get(path, name string) ([]byte, error) {
	return nil, nil
}

