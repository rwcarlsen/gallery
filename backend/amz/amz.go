
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

func (lb *Backend) makeBucket(path string) (bucket *s3.Bucket, bpath string, err error) {
	items := strings.Split(path, "/")
	if len(items) == 0 {
		return nil, "", errors.New("amz: path is too short")
	}
	bucket = lb.s3link.Bucket(items[0])
	bpath = pth.Join(items[1:]...)

	if _, err := b.List("", "", "", 1); err != nil {
		if s3err, ok := err.s3.Error; ok && s3err.Code == "NoSuchBucket" {
			if err = b.PutBucket(s3.Private); err != nil {
				log.Println(err)
				return err
			} else {
				log.Printf("Created bucket '%v'", b.Name)
			}
		}
		log.Println(err)
		return err
	}
	return bucket, bpath, nil
}

func (lb *Backend) Put(path, name string, data []byte) error {
	bucket, bpath, err := lb.makeBucket(path)
	if err != nil {
		return err
	}
	fullPath := pth.Join(bpath, name)

	contType := http.DetectContentType(data)
	return bucket.Put(fullPath, data, contType, s3.Private)
}

func (lb *Backend) Exists(path, name string) bool {
	bucket, bpath, err := lb.makeBucket(path)
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

// ListN returns up to n items contained in the bucket/prefix defined by path.
// There is no limit to n.  When n=0 up to 1000 results are returned.
func (lb *Backend) ListN(path string, n int) ([]string, error) {
	bucket, bpath, err := lb.makeBucket(path)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	marker := ""
	for {
		result, err := bucket.List(bpath, "", marker, n)
		if err != nil {
			return nil, err
		}

		var k s3.Key
		for _, k = range result.Contents {
			name := strings.Split(k.Key, "/")[1]
			names = append(names, name)
		}

		if result.IsTruncated {
			marker = k.Key
		} else {
			break
		}
	}
	return names, nil
}

func (lb *Backend) Get(path, name string) ([]byte, error) {
	bucket, bpath, err := lb.makeBucket(path)
	if err != nil {
		return nil, err
	}
	fullPath := pth.Join(bpath, name)

	return bucket.Get(fullPath)
}


