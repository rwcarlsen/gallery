
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
	buckets map[string]*s3.Bucket
}

func New(auth aws.Auth, region aws.Region) *Backend {
	return &Backend{
		s3link: s3.New(auth, region),
		buckets: make(map[string]*s3.Bucket),
	}
}

const (
	NoSuchBucket = "NoSuchBucket"
)

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

	log.Printf("GetObject %v/%v", bucket.Name, bpath)
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
		log.Printf("ListBucket %v/%v", bucket.Name, bpath)
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

	log.Printf("GetObject %v/%v", bucket.Name, bpath)
	return bucket.Get(fullPath)
}


