package client

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"golang.org/x/sys/windows"
)

type ObjectClient struct {
	S3Client   *s3.Client
	NumWorkers int
	BreakPoint int
}

var endpoint string
var region string
var accessKey string
var secretKey string

func NewObjectClient() *ObjectClient {
	cfg := aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		Region:      region,
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           endpoint,
					SigningRegion: region,
				}, nil
			},
		),
	}
	client := s3.NewFromConfig(cfg)
	return &ObjectClient{
		S3Client:   client,
		NumWorkers: 1,
		BreakPoint: 0,
	}
}

func enableANSI() {
	stdout := windows.Handle(os.Stdout.Fd())
	var originalMode uint32
	windows.GetConsoleMode(stdout, &originalMode)
	windows.SetConsoleMode(stdout, originalMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}

func (client *ObjectClient) ListBuckets() (string, error) {
	result, err := client.S3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return "", err
	}
	var results []string
	for _, bucket := range result.Buckets {
		results = append(results, aws.ToString(bucket.Name))
	}
	return strings.Join(results, "\n"), nil
}

func (client *ObjectClient) UploadFiles(bucket, s3Folder, localDir string) (string, error) {
	enableANSI()
	var totalCnt int = 0
	var uploadedCnt int = 0
	workersDo := make([]int, client.NumWorkers)
	uploadingFiles := make([]string, client.NumWorkers)
	if bucket == "" {
		return "please select bucket.", nil
	}

	fmt.Print("Caculating... \n")
	defer fmt.Println()

	files, err := os.ReadDir(localDir)

	if err != nil {
		return "", err
	}
	totalCnt = len(files)
	fmt.Printf("TotalCnt : %d\n", totalCnt)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		chars := []string{"|", "/", "-", "\\"}
		i := 0
		for range ticker.C {
			// 메인 업로드 상태
			fmt.Printf("\rUploading... %s %d/%d\n", chars[i], uploadedCnt, totalCnt)

			// 각 워커의 상태를 새로운 줄에 표시
			for workerID := 0; workerID < client.NumWorkers; workerID++ {
				fmt.Printf("\rWorker#%d: %d files uploaded; now uploading file is %s\n", workerID, workersDo[workerID], uploadingFiles[workerID])
			}

			// 커서를 다시 위로 올림
			fmt.Printf("\033[%dA", client.NumWorkers+1)

			i = (i + 1) % len(chars)
		}
	}()

	fileChan := make(chan os.DirEntry, totalCnt)
	var wg sync.WaitGroup

	for i := 0; i < client.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for file := range fileChan {
				if file.IsDir() || filepath.Ext(file.Name()) != ".pdf" {
					continue
				}
				localFilePath := filepath.Join(localDir, file.Name())
				err := client.upload(bucket, s3Folder, localFilePath, file)
				if err != nil {
					log.Printf("[Worker %d] ❌ Failed to upload %s: %v", workerID, file.Name(), err)
					continue
				}
				uploadedCnt++
				workersDo[workerID]++
				uploadingFiles[workerID] = file.Name()
			}
		}(i)
	}

	var start bool = false
	for i, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".pdf" {
			continue
		}

		if i == client.BreakPoint {
			start = true
		}

		if !start {
			continue
		}

		fileChan <- file
	}
	close(fileChan)
	wg.Wait()

	return "🚀 모든 PDF 업로드 완료!", nil
}

func (client *ObjectClient) upload(bucket, s3Folder, filePath string, file os.DirEntry) error {
	s3Key := s3Folder + "/" + file.Name()

	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("❌ Failed to open file %s : %v \n", file.Name(), err)
		return err
	}

	defer f.Close()

	_, err = client.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(s3Key),
		Body:        f,
		ContentType: aws.String("application/pdf"),
	})

	return err

}

func (client *ObjectClient) List(bucket, folder string) (string, error) {
	contents, err := client.listS3Files(bucket, folder)
	if err != nil {
		return "", err
	}

	for _, file := range contents {
		fmt.Printf("📂 %s | 🕒 %s\n", *file.Key, file.LastModified.Format(time.RFC3339))
	}

	return "", nil
}

func (client *ObjectClient) Count(bucket, folder string) (string, error) {
	contents, err := client.countS3Files(bucket, folder)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return fmt.Sprintf("Bucket : %s, Dir : %s, Total Count : %d\n", bucket, folder, len(contents)), nil
}

func (client *ObjectClient) listS3Files(bucket, s3Folder string) ([]types.Object, error) {
	s3Key := s3Folder + "/"
	resp, err := client.S3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(s3Key),
	})
	if err != nil {
		return nil, err
	}
	return resp.Contents, nil
}

func (client *ObjectClient) countS3Files(bucket, s3Folder string) ([]types.Object, error) {
	s3Key := s3Folder + "/"

	var allFiles []types.Object
	var continuationToken *string

	for {
		resp, err := client.S3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(s3Key),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, err
		}

		allFiles = append(allFiles, resp.Contents...)

		if !*resp.IsTruncated {
			break
		}

		continuationToken = resp.NextContinuationToken
	}
	fmt.Printf("%s  %s\n", *allFiles[len(allFiles)-1].Key, allFiles[len(allFiles)-1].LastModified.Format(time.RFC3339))
	return allFiles, nil
}
