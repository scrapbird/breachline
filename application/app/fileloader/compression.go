package fileloader

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/ulikunitz/xz"
)

// CompressionType represents the compression format of a file
type CompressionType int

const (
	CompressionNone CompressionType = iota
	CompressionGzip
	CompressionBzip2
	CompressionXZ
)

// String returns the string representation of CompressionType
func (ct CompressionType) String() string {
	switch ct {
	case CompressionGzip:
		return "gzip"
	case CompressionBzip2:
		return "bzip2"
	case CompressionXZ:
		return "xz"
	default:
		return "none"
	}
}

// Magic byte signatures for compression detection
var (
	// Gzip magic bytes: 1f 8b
	gzipMagic = []byte{0x1f, 0x8b}
	// Bzip2 magic bytes: 42 5a 68 ("BZh")
	bzip2Magic = []byte{0x42, 0x5a, 0x68}
	// XZ magic bytes: fd 37 7a 58 5a 00
	xzMagic = []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}
)

// DecompressionResult contains the decompressed data and any warning
type DecompressionResult struct {
	Data    []byte
	Warning string // Non-empty if decompression was incomplete
}

// DetectCompressionByMagic reads the first few bytes of a file and detects compression type
func DetectCompressionByMagic(filePath string) (CompressionType, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return CompressionNone, err
	}
	defer f.Close()

	// Read enough bytes to detect any compression format
	// XZ has the longest magic (6 bytes)
	header := make([]byte, 6)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return CompressionNone, err
	}

	// Check for each compression type
	if n >= 2 && bytes.HasPrefix(header, gzipMagic) {
		return CompressionGzip, nil
	}
	if n >= 3 && bytes.HasPrefix(header, bzip2Magic) {
		return CompressionBzip2, nil
	}
	if n >= 6 && bytes.HasPrefix(header, xzMagic) {
		return CompressionXZ, nil
	}

	return CompressionNone, nil
}

// DecompressFile reads a compressed file and returns the decompressed data.
// If decompression fails mid-stream, it returns partial data with a warning message.
func DecompressFile(filePath string, compressionType CompressionType) (*DecompressionResult, error) {
	if compressionType == CompressionNone {
		// No compression, just read the file
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		return &DecompressionResult{Data: data}, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader
	var decompressErr error

	switch compressionType {
	case CompressionGzip:
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader

	case CompressionBzip2:
		reader = bzip2.NewReader(f)

	case CompressionXZ:
		xzReader, err := xz.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		reader = xzReader

	default:
		return nil, fmt.Errorf("unsupported compression type: %v", compressionType)
	}

	// Read all data, capturing any mid-stream errors
	var buf bytes.Buffer
	_, decompressErr = io.Copy(&buf, reader)

	result := &DecompressionResult{
		Data: buf.Bytes(),
	}

	// If there was an error during decompression but we got some data,
	// treat it as a partial success with a warning
	if decompressErr != nil {
		if len(result.Data) > 0 {
			result.Warning = fmt.Sprintf("Decompression incomplete: %v. Some data may be missing.", decompressErr)
		} else {
			return nil, fmt.Errorf("decompression failed: %w", decompressErr)
		}
	}

	return result, nil
}

// GetDecompressingReader returns a reader that decompresses the file on-the-fly.
// This is useful for streaming scenarios, but note that most file formats
// in this application require full file loading anyway.
func GetDecompressingReader(filePath string, compressionType CompressionType) (io.ReadCloser, error) {
	if compressionType == CompressionNone {
		return os.Open(filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	switch compressionType {
	case CompressionGzip:
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return &decompressingReadCloser{reader: gzReader, file: f}, nil

	case CompressionBzip2:
		bzReader := bzip2.NewReader(f)
		return &decompressingReadCloser{reader: bzReader, file: f}, nil

	case CompressionXZ:
		xzReader, err := xz.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		return &decompressingReadCloser{reader: xzReader, file: f}, nil

	default:
		f.Close()
		return nil, fmt.Errorf("unsupported compression type: %v", compressionType)
	}
}

// decompressingReadCloser wraps a decompressing reader and the underlying file
type decompressingReadCloser struct {
	reader io.Reader
	file   *os.File
}

func (d *decompressingReadCloser) Read(p []byte) (n int, err error) {
	return d.reader.Read(p)
}

func (d *decompressingReadCloser) Close() error {
	// Close the gzip reader if it's a Closer
	if closer, ok := d.reader.(io.Closer); ok {
		closer.Close()
	}
	return d.file.Close()
}
