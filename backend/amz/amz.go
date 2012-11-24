
package amz

import (
	"log"
	"errors"
	pth "path"
	"strings"
	"launchpad.net/goamz/s3"
	"launchpad.net/goamz/aws"
	"net/http"
)

type Backend struct {
	s3link *s3.S3
}

func New(auth aws.Auth, region aws.Region) *Backend {
	return &Backend{
		s3link: s3.New(auth, region),
	}
}

func (lb *Backend) splitBucket(path string) (bucket *s3.Bucket, bpath string, err error) {
	items := strings.Split(path, "/")
	if len(items) == 0 {
		return nil, "", errors.New("amz: path is too short")
	}
	bucket = lb.s3link.Bucket(items[0])
	bpath = pth.Join(items[1:]...)
	lb.initBucket(bucket)

	return bucket, bpath, nil
}

func (lb *Backend) initBucket(b *s3.Bucket) {
	err := b.PutBucket(s3.Private)
	if err != nil {
		log.Println(err)
	}
}

func (lb *Backend) Put(path, name string, data []byte) error {
	bucket, bpath, err := lb.splitBucket(path)
	if err != nil {
		return err
	}
	fullPath := pth.Join(bpath, name)

	contType := http.DetectContentType(data)
	return bucket.Put(fullPath, data, contType, s3.Private)
}

func (lb *Backend) Exists(path, name string) bool {
	bucket, bpath, err := lb.splitBucket(path)
	if err != nil {
		return false
	}
	fullPath := pth.Join(bpath, name)

	_, err = bucket.Get(fullPath)
	if err != nil {
		return false
	}

	return true
}

func (lb *Backend) ListN(path string, n int) ([]string, error) {
	bucket, bpath, err := lb.splitBucket(path)
	if err != nil {
		return nil, err
	}

	result, err := bucket.List(bpath, "", "", n)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	for _, k := range result.Contents {
		name := strings.Split(k.Key, "/")[1]
		names = append(names, name)
	}
	return names, nil
}

func (lb *Backend) Get(path, name string) ([]byte, error) {
	bucket, bpath, err := lb.splitBucket(path)
	if err != nil {
		return nil, err
	}
	fullPath := pth.Join(bpath, name)

	return bucket.Get(fullPath)
}


