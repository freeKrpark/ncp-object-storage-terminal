package command

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/freeKrpark/ncp-object-storage-terminal/client"
)

const (
	colorReset = "\033[0m"
	colorBlue  = "\033[34m"
	colorGreen = "\033[32m"
)

type Command struct {
	Path   string
	Client *client.ObjectClient
	bucket string
	s3Dir  string
}

func (cmd *Command) HandleExit(_ string) (string, bool) {
	return "", true
}

func (cmd *Command) HandleLS(_ string) (string, bool) {
	text, err := listDir(cmd.Path)
	if err != nil {
		return "Failed to list directory", false
	}
	return text, false
}

func (cmd *Command) HandleCD(text string) (string, bool) {
	texts := strings.SplitN(text, "cd ", 2)
	if len(texts) != 2 {
		return "Invalid cd command", false
	}

	updatedPath, err := changeDirectory(cmd.Path, texts[1])
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Sprintf("path: %s does not exist", updatedPath), false
		}
		return fmt.Sprintf("Error: %v", err), false
	}
	cmd.Path = updatedPath
	return "", false
}

func (cmd *Command) HandleShowBuckets(_ string) (string, bool) {
	text, err := cmd.Client.ListBuckets()
	if err != nil {
		return "Failed to list buckets", false
	}
	return text, false
}

func (cmd *Command) HandleUseBucket(text string) (string, bool) {
	texts := strings.SplitN(text, "use ", 2)
	if len(texts) != 2 {
		return "Invalid use command", false
	}
	cmd.bucket = texts[1]
	return fmt.Sprintf("Bucket %s is selected.", cmd.bucket), false
}

func (cmd *Command) HandleSetS3Dir(text string) (string, bool) {
	texts := strings.SplitN(text, "set ", 2)
	if len(texts) != 2 {
		return "Invalid set command", false
	}
	cmd.s3Dir = texts[1]
	return fmt.Sprintf("S3Dir %s is set.", cmd.s3Dir), false
}

func (cmd *Command) HandleStartUpload(_ string) (string, bool) {
	text, err := cmd.Client.UploadFiles(cmd.bucket, cmd.s3Dir, cmd.Path)
	if err != nil {
		return "Faield to upload", false
	}
	return text, false
}

func (cmd *Command) HandleListBucket(_ string) (string, bool) {
	text, err := cmd.Client.List(cmd.bucket, cmd.s3Dir)
	if err != nil {
		return "Failed to list bucket", false
	}
	return text, false
}

func (cmd *Command) HandleCountBucket(_ string) (string, bool) {
	text, err := cmd.Client.Count(cmd.bucket, cmd.s3Dir)
	if err != nil {
		return "Failed to count bucket", false
	}
	return text, false
}

func (cmd *Command) HandleSetWorkers(text string) (string, bool) {
	texts := strings.SplitN(text, "workers ", 2)
	if len(texts) != 2 {
		return "Invalid set workers command", false
	}

	workers, err := strconv.Atoi(texts[1])
	if err != nil {
		return "Failed to set workers: invalid number", false
	}

	cmd.Client.NumWorkers = workers
	return fmt.Sprintf("Worker's number is %d.", workers), false
}

func (cmd *Command) HandleSetBreakPoint(text string) (string, bool) {
	texts := strings.SplitN(text, "breakpoint ", 2)
	if len(texts) != 2 {
		return "Invalid set breakPoint command", false
	}

	breakPoint, err := strconv.Atoi(texts[1])
	if err != nil {
		return "Failed to set breakPoint: invalid number", false
	}

	cmd.Client.BreakPoint = breakPoint
	return fmt.Sprintf("BreakPoint is %d.", breakPoint), false
}

func existsDir(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return false, err
}

func listDir(path string) (string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var output []string
	for _, file := range files {
		if file.IsDir() {
			output = append(output, colorBlue+file.Name()+colorReset)
		} else {
			output = append(output, file.Name())
		}
	}
	return strings.Join(output, "\n"), nil
}

func changeDirectory(path, newPath string) (string, error) {
	var dir string
	if strings.HasPrefix(newPath, "/") || strings.HasPrefix(newPath, `\`) {
		dir = newPath
	} else {
		dir = filepath.Join(path, newPath)
	}

	exists, err := existsDir(dir)
	if err != nil {
		return dir, err
	}
	if !exists {
		return dir, fmt.Errorf("not exists")
	}
	return dir, nil
}
