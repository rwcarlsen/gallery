
package amz

import (
	"fmt"
	"log"
	"errors"
	pth "path"
	"strings"
	//"launchpad.net/goamz/s3"
	"github.com/rwcarlsen/goamz/s3"
	"launchpad.net/goamz/aws"
	"net/http"
)

const (
	NoSuchBucket = "NoSuchBucket"
	maxRetries = 4
)

type Backend struct {
	s3link *s3.S3
	buckets map[string]*s3.Bucket
	DbName string
}

func New(auth aws.Auth, region aws.Region) *Backend {
	return &Backend{
		s3link: s3.New(auth, region),
		buckets: make(map[string]*s3.Bucket),
	}
}

func (lb *Backend) Name() string {
	return lb.DbName
}

func (lb *Backend) makeBucket(path string) (bucket *s3.Bucket, bpath string, err error) {
	items := strings.Split(path, "/")
	if len(items) == 0 {
		return nil, "", errors.New("amz: path is too short")
	}
	bname := items[0]
	bpath = pth.Join(items[1:]...)
	if b, ok := lb.buckets[bname]; ok {
		return b, bpath, nil
	}

	b := lb.s3link.Bucket(bname)
	lb.buckets[bname] = b

	// check if bucket exists and post it if necessary
	log.Printf("ListBucket %v/%v", b.Name, bpath)
	if _, err := b.List("", "", "", 1); err != nil {
		if s3err, ok := err.(*s3.Error); ok && s3err.Code == NoSuchBucket {
			if err = b.PutBucket(s3.Private); err != nil {
				log.Println(err)
				return nil, "", err
			}
			log.Printf("Created bucket '%v'", b.Name)
		} else {
			log.Println(err)
			return nil, "", err
		}
	}
	return b, bpath, nil
}

func (lb *Backend) Put(path string, data []byte) error {
	bucket, bpath, err := lb.makeBucket(path)
	if err != nil {
		return err
	}

	contType := http.DetectContentType(data)

	for i := 0; i < maxRetries; i++ {
		log.Printf("PutObject %v/%v", bucket.Name, bpath)
		if err = bucket.Put(bpath, data, contType, s3.Private); err == nil {
			break
		}
		log.Printf("PutObject failed %v/%v", bucket.Name, bpath)
	}
	return err
}

func (lb *Backend) Exists(path string) bool {
	bucket, bpath, err := lb.makeBucket(path)
	if err != nil {
		return false
	}

	log.Printf("GetObject %v/%v", bucket.Name, bpath)
	_, err = bucket.Get(bpath)
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
	failed := 0
	for failed < maxRetries {
		result, err := bucket.List(bpath, "", marker, n)
		log.Printf("ListBucket %v/%v", bucket.Name, bpath)
		if err != nil {
			log.Printf("ListBucket failed %v/%v", bucket.Name, bpath)
			failed++
			continue
		}

		var k s3.Key
		for _, k = range result.Contents {
			names = append(names, pth.Join(bucket.Name, k.Key))
		}

		if result.IsTruncated  && len(result.Contents) < n {
			marker = k.Key
		} else {
			break
		}
	}

	if failed == maxRetries {
		return nil, fmt.Errorf("ListBucket FAILED!!! %v/%v", bucket.Name, bpath)
	}
	return names, nil
}

func (lb *Backend) Get(path string) ([]byte, error) {
	bucket, bpath, err := lb.makeBucket(path)
	if err != nil {
		return nil, err
	}

	var data []byte
	for i := 0; i < maxRetries; i++ {
		if data, err = bucket.Get(bpath); err == nil {
			log.Printf("GetObject %v/%v", bucket.Name, bpath)
			return data, err
		}
		log.Printf("GetObject failed %v/%v", bucket.Name, bpath)
	}
	return nil, err
}


