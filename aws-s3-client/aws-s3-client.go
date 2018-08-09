package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/cyberdelia/aws"
)

type retryFunc func() (*http.Response, error)

func retry(f retryFunc, r int) (resp *http.Response, err error) {
	for i := 0; i < r; i++ {
		if resp, err = f(); err == nil {
			return resp, nil
		}
	}
	return nil, err
}

func retryNoBody(c *http.Client, req *http.Request) retryFunc {
	if req.GetBody != nil {
		panic("request should not contain a body")
	}
	return func() (resp *http.Response, err error) {
		if resp, err = c.Do(req); err != nil {
			return nil, err
		}
		// Retry on internal errors as recommended.
		// http://docs.aws.amazon.com/AmazonS3/latest/dev/ErrorBestPractices.html#UsingErrorsRetry
		if resp.StatusCode == 500 {
			return nil, fmt.Errorf("failed to get from AWS S3, status code %d", resp.StatusCode)
		}
		return resp, nil
	}
}

func main() {
	var region, accessKey, secretKey, bucket, path string
	flag.StringVar(&region, "region", "ap-northeast-2", "AWS region default:Seoul")
	flag.StringVar(&accessKey, "access-key", "", "AWS access key of credential")
	flag.StringVar(&secretKey, "secret-key", "", "AWS secret key of credential")
	flag.StringVar(&bucket, "bucket", "", "AWS bucket")
	flag.StringVar(&path, "path", "", "leaf path of URL")
	flag.Parse()

	signer := &aws.V4Signer{
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Service:   "s3",
	}

	client := &http.Client{
		Transport: signer.Transport(),
	}

	req, err := http.NewRequest("GET", "http://"+bucket+".s3.amazonaws.com/"+path, nil)
	if err != nil {
		log.Fatal(err)
	}

	const retries = 3
	resp, err := retry(retryNoBody(client, req), retries)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("response code : %d\n", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	log.Print(string(body))
}
