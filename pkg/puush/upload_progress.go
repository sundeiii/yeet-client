package puush

import (
	"io"
	"os"
)

type ProgressReader struct {
	io.ReadCloser
	Total      int64
	Current    int64
	OnProgress func(percentage float64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.ReadCloser.Read(p)
	pr.Current += int64(n)

	if pr.Total > 0 && pr.OnProgress != nil {
		percentage := float64(pr.Current) / float64(pr.Total) * 100
		pr.OnProgress(percentage)
	}

	return n, err
}

func NewProgressReader(reader io.ReadCloser, total int64, onProgress func(percentage float64)) *ProgressReader {
	return &ProgressReader{
		ReadCloser: reader,
		Total:      total,
		OnProgress: onProgress,
	}
}

func NewProgressReaderFromFile(path string, onProgress func(percentage float64)) (*ProgressReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &ProgressReader{
		ReadCloser: file,
		Total:      fileInfo.Size(),
		OnProgress: onProgress,
	}, nil
}
