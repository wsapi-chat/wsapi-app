package whatsapp

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
)

// ResizeImageIfNeeded resizes an image if it is larger than 640 pixels in any
// direction while maintaining the aspect ratio. Returns the resized image as
// a byte slice.
func ResizeImageIfNeeded(imageData []byte) ([]byte, error) {
	if len(imageData) == 0 {
		return nil, fmt.Errorf("empty image data")
	}

	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %v", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	maxDimension := 640
	if width <= maxDimension && height <= maxDimension {
		return imageData, nil
	}

	var newWidth, newHeight int
	if width > height {
		newWidth = maxDimension
		newHeight = int(float64(height) * float64(maxDimension) / float64(width))
	} else {
		newHeight = maxDimension
		newWidth = int(float64(width) * float64(maxDimension) / float64(height))
	}

	resizedImg := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	var buf bytes.Buffer

	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(&buf, resizedImg)
	default:
		err = jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 85})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode resized image: %v", err)
	}

	return buf.Bytes(), nil
}
