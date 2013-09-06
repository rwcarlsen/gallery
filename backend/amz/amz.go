package amz

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	pth "path"
	"strings"
	//"github.com/rwcarlsen/goamz/s3"
	"github.com/rwcarlsen/goamz/aws"
	"github.com/rwcarlsen/goamz/s3"
	"io"
	"io/ioutil"
	"net/http"
)

const (
	NoSuchBucket = "NoSuchBucket"
	maxRetries   = 4
)

type Backend struct {
	s3link  *s3.S3
	buckets map[string]*s3.Bucket
}

func New(auth aws.Auth, region aws.Region) *Backend {
	return &Backend{
		s3link:  s3.New(auth, region),
		buckets: make(map[string]*s3.Bucket),
	}
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

func (lb *Backend) Close() error { return nil }

func (lb *Backend) Put(key string, r io.Reader) (n int64, err error) {
	bucket, bpath, err := lb.makeBucket(key)
	if err != nil {
		return 0, err
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return 0, err
	}

	contType := http.DetectContentType(data)

	for i := 0; i < maxRetries; i++ {
		if err = bucket.Put(bpath, data, contType, s3.Private); err == nil {
			log.Printf("PutObject %v/%v", bucket.Name, bpath)
			return int64(len(data)), nil
		}
		log.Printf("PutObject failed %v/%v: %v", bucket.Name, bpath, err)
	}
	return 0, err
}

func (lb *Backend) Del(key string) error {
	bucket, bpath, err := lb.makeBucket(key)
	if err != nil {
		return err
	}

	for i := 0; i < maxRetries; i++ {
		if err = bucket.Del(bpath); err == nil {
			log.Printf("DelObject %v/%v", bucket.Name, bpath)
			return nil
		}
		log.Printf("DelObject failed %v/%v: %v", bucket.Name, bpath, err)
	}
	return err
}

// Enumerate returns up to limit items contained in the bucket/prefix defined by path.
// There is no limit to n.  When n=0 up to 1000 results are returned.
func (lb *Backend) Enumerate(prefix string, limit int) ([]string, error) {
	bucket, bpath, err := lb.makeBucket(prefix)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	marker := ""
	failed := 0
	for failed < maxRetries {
		result, err := bucket.List(bpath, "", marker, limit)
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

		if result.IsTruncated && len(result.Contents) < limit {
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

func (lb *Backend) Get(key string) (io.ReadCloser, error) {
	bucket, bpath, err := lb.makeBucket(key)
	if err != nil {
		return nil, err
	}

	var data []byte
	for i := 0; i < maxRetries; i++ {
		if data, err = bucket.Get(bpath); err == nil {
			log.Printf("GetObject %v/%v", bucket.Name, bpath)
			return readCloser{bytes.NewBuffer(data)}, nil
		}
		log.Printf("GetObject failed %v/%v", bucket.Name, bpath)
	}
	return nil, err
}

type readCloser struct{ *bytes.Buffer }

func (_ readCloser) Close() error { return nil }
