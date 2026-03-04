package event

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
)

// MediaDownloadInfo represents the essential media information needed for download.
// This is separate from the event MediaInfo struct; it holds the binary fields
// required to reconstruct a whatsmeow download request.
type MediaDownloadInfo struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	MimeType      string
	FileName      string
	MediaType     string
	FileLength    uint64
}

// EncodeMediaID creates a compact, URL-safe string containing all media download information.
// Format: [version(1)][url_len(2)][directpath_len(2)][mediakey_len(1)][filesha256_len(1)][fileencsha256_len(1)][mimetype_len(1)][filename_len(1)][mediatype_len(1)][filelength(8)][data...]
func EncodeMediaID(info MediaDownloadInfo) (string, error) {
	var buf bytes.Buffer

	// Version byte for future compatibility
	buf.WriteByte(1)

	// Convert strings to bytes
	urlBytes := []byte(info.URL)
	directPathBytes := []byte(info.DirectPath)
	mimeTypeBytes := []byte(info.MimeType)
	fileNameBytes := []byte(info.FileName)
	mediaTypeBytes := []byte(info.MediaType)

	// Validate lengths
	if len(urlBytes) > 65535 || len(directPathBytes) > 65535 {
		return "", fmt.Errorf("URL or DirectPath too long")
	}
	if len(info.MediaKey) > 255 || len(info.FileSHA256) > 255 || len(info.FileEncSHA256) > 255 ||
		len(mimeTypeBytes) > 255 || len(fileNameBytes) > 255 || len(mediaTypeBytes) > 255 {
		return "", fmt.Errorf("field too long")
	}

	// Write lengths
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(urlBytes)))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(directPathBytes)))
	buf.WriteByte(byte(len(info.MediaKey)))
	buf.WriteByte(byte(len(info.FileSHA256)))
	buf.WriteByte(byte(len(info.FileEncSHA256)))
	buf.WriteByte(byte(len(mimeTypeBytes)))
	buf.WriteByte(byte(len(fileNameBytes)))
	buf.WriteByte(byte(len(mediaTypeBytes)))

	// Write FileLength as uint64
	_ = binary.Write(&buf, binary.LittleEndian, info.FileLength)

	// Write data
	buf.Write(urlBytes)
	buf.Write(directPathBytes)
	buf.Write(info.MediaKey)
	buf.Write(info.FileSHA256)
	buf.Write(info.FileEncSHA256)
	buf.Write(mimeTypeBytes)
	buf.Write(fileNameBytes)
	buf.Write(mediaTypeBytes)

	// Compress the data
	var compressed bytes.Buffer
	gzWriter := gzip.NewWriter(&compressed)
	if _, err := gzWriter.Write(buf.Bytes()); err != nil {
		return "", fmt.Errorf("failed to compress data: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close compressor: %v", err)
	}

	// Encode as URL-safe base64
	encoded := base64.URLEncoding.EncodeToString(compressed.Bytes())

	return encoded, nil
}

// DecodeMediaID decodes a media ID string back to MediaDownloadInfo.
func DecodeMediaID(encoded string) (MediaDownloadInfo, error) {
	// Decode from URL-safe base64
	compressed, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to decode base64: %v", err)
	}

	// Decompress the data
	gzReader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to create decompressor: %v", err)
	}
	defer gzReader.Close() //nolint:errcheck

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to decompress data: %v", err)
	}

	buf := bytes.NewReader(decompressed)

	// Read version
	version, err := buf.ReadByte()
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read version: %v", err)
	}
	if version != 1 {
		return MediaDownloadInfo{}, fmt.Errorf("unsupported version: %d", version)
	}

	// Read lengths
	var urlLen, directPathLen uint16
	if err := binary.Read(buf, binary.LittleEndian, &urlLen); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read URL length: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &directPathLen); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read DirectPath length: %v", err)
	}

	mediaKeyLen, err := buf.ReadByte()
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read MediaKey length: %v", err)
	}

	fileSHA256Len, err := buf.ReadByte()
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read FileSHA256 length: %v", err)
	}

	fileEncSHA256Len, err := buf.ReadByte()
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read FileEncSHA256 length: %v", err)
	}

	mimeTypeLen, err := buf.ReadByte()
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read MimeType length: %v", err)
	}

	fileNameLen, err := buf.ReadByte()
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read FileName length: %v", err)
	}

	mediaTypeLen, err := buf.ReadByte()
	if err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read MediaType length: %v", err)
	}

	// Read FileLength
	var fileLength uint64
	if err := binary.Read(buf, binary.LittleEndian, &fileLength); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read FileLength: %v", err)
	}

	// Read data fields
	urlBytes := make([]byte, urlLen)
	if _, err := io.ReadFull(buf, urlBytes); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read URL: %v", err)
	}

	directPathBytes := make([]byte, directPathLen)
	if _, err := io.ReadFull(buf, directPathBytes); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read DirectPath: %v", err)
	}

	mediaKey := make([]byte, mediaKeyLen)
	if _, err := io.ReadFull(buf, mediaKey); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read MediaKey: %v", err)
	}

	fileSHA256 := make([]byte, fileSHA256Len)
	if _, err := io.ReadFull(buf, fileSHA256); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read FileSHA256: %v", err)
	}

	fileEncSHA256 := make([]byte, fileEncSHA256Len)
	if _, err := io.ReadFull(buf, fileEncSHA256); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read FileEncSHA256: %v", err)
	}

	mimeTypeBytes := make([]byte, mimeTypeLen)
	if _, err := io.ReadFull(buf, mimeTypeBytes); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read MimeType: %v", err)
	}

	fileNameBytes := make([]byte, fileNameLen)
	if _, err := io.ReadFull(buf, fileNameBytes); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read FileName: %v", err)
	}

	mediaTypeBytes := make([]byte, mediaTypeLen)
	if _, err := io.ReadFull(buf, mediaTypeBytes); err != nil {
		return MediaDownloadInfo{}, fmt.Errorf("failed to read MediaType: %v", err)
	}

	return MediaDownloadInfo{
		URL:           string(urlBytes),
		DirectPath:    string(directPathBytes),
		MediaKey:      mediaKey,
		FileSHA256:    fileSHA256,
		FileEncSHA256: fileEncSHA256,
		MimeType:      string(mimeTypeBytes),
		FileName:      string(fileNameBytes),
		MediaType:     string(mediaTypeBytes),
		FileLength:    fileLength,
	}, nil
}

// createMediaID is a helper function used by media projection functions.
// It extracts common media fields and creates an encoded ID.
func createMediaID(url, directPath, mimeType, fileName, mediaType string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) string {
	safeMediaKey := mediaKey
	if safeMediaKey == nil {
		safeMediaKey = []byte{}
	}

	safeFileSHA256 := fileSHA256
	if safeFileSHA256 == nil {
		safeFileSHA256 = []byte{}
	}

	safeFileEncSHA256 := fileEncSHA256
	if safeFileEncSHA256 == nil {
		safeFileEncSHA256 = []byte{}
	}

	info := MediaDownloadInfo{
		URL:           url,
		DirectPath:    directPath,
		MediaKey:      safeMediaKey,
		FileSHA256:    safeFileSHA256,
		FileEncSHA256: safeFileEncSHA256,
		MimeType:      mimeType,
		FileName:      fileName,
		MediaType:     mediaType,
		FileLength:    fileLength,
	}

	encoded, err := EncodeMediaID(info)
	if err != nil {
		return ""
	}

	return encoded
}
