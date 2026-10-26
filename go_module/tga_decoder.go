package main

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
)

const (
	// Foundry displays textures as previews and thumbnails. A 4096x4096
	// RGBA image already consumes 64 MiB before renderer copies, so larger
	// decoded assets are not useful enough to justify process-wide OOM risk.
	maxRasterPixels      = 4096 * 4096
	maxRasterDecodedSize = maxRasterPixels * 4
	maxRasterInputSize   = maxRasterDecodedSize + 1<<20
)

var (
	errInvalidTGA              = errors.New("tga: invalid format")
	errInvalidRasterDimensions = errors.New("raster: invalid dimensions")
	errRasterTooLarge          = errors.New("raster: decoded image exceeds 16777216 pixels or 64 MiB")
	errRasterInputTooLarge     = errors.New("raster: encoded input exceeds 65 MiB")
)

func readRasterBytes(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxRasterInputSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRasterInputSize {
		return nil, errRasterInputTooLarge
	}
	return data, nil
}

func checkedRasterPixelCount(width, height int) (int, error) {
	if width <= 0 || height <= 0 {
		return 0, errInvalidRasterDimensions
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/height {
		return 0, errRasterTooLarge
	}
	pixels := width * height
	if pixels > maxRasterPixels || pixels > maxRasterDecodedSize/4 {
		return 0, errRasterTooLarge
	}
	return pixels, nil
}

type tgaHeader struct {
	idLength      int
	colorMapType  byte
	imageType     byte
	colorMapFirst int
	colorMapLen   int
	colorMapDepth int
	width         int
	height        int
	pixelDepth    int
	descriptor    byte
}

// decodeTGA decodes the uncompressed and RLE true-color, grayscale, and
// color-mapped TGA variants used by JKA/MBII. It intentionally does not call
// image.RegisterFormat: the former dependency registered TGA with an empty
// magic signature, causing Fyne to send every PNG/JPEG resource to the TGA
// decoder first.
func decodeTGA(data []byte) (image.Image, error) {
	if len(data) > maxRasterInputSize {
		return nil, errRasterInputTooLarge
	}
	if len(data) < 18 {
		return nil, errInvalidTGA
	}
	h := tgaHeader{
		idLength:      int(data[0]),
		colorMapType:  data[1],
		imageType:     data[2],
		colorMapFirst: int(binary.LittleEndian.Uint16(data[3:5])),
		colorMapLen:   int(binary.LittleEndian.Uint16(data[5:7])),
		colorMapDepth: int(data[7]),
		width:         int(binary.LittleEndian.Uint16(data[12:14])),
		height:        int(binary.LittleEndian.Uint16(data[14:16])),
		pixelDepth:    int(data[16]),
		descriptor:    data[17],
	}
	baseType := h.imageType & 7
	rle := h.imageType&8 != 0
	if h.imageType != baseType && h.imageType != baseType|8 {
		return nil, errInvalidTGA
	}
	if h.width <= 0 || h.height <= 0 || (baseType != 1 && baseType != 2 && baseType != 3) {
		return nil, errInvalidTGA
	}
	pixelCount, err := checkedRasterPixelCount(h.width, h.height)
	if err != nil {
		return nil, err
	}
	if (baseType == 1) != (h.colorMapType == 1) {
		return nil, errInvalidTGA
	}

	pos := 18 + h.idLength
	if pos < 18 || pos > len(data) {
		return nil, errInvalidTGA
	}

	var palette []color.NRGBA
	if baseType == 1 {
		entryBytes := (h.colorMapDepth + 7) / 8
		if h.colorMapLen <= 0 || entryBytes <= 0 || h.colorMapLen > (len(data)-pos)/entryBytes {
			return nil, errInvalidTGA
		}
		palette = make([]color.NRGBA, h.colorMapLen)
		for i := range palette {
			pixel, ok := decodeTGAColor(data[pos:pos+entryBytes], h.colorMapDepth, h.colorMapDepth == 32)
			if !ok {
				return nil, errInvalidTGA
			}
			palette[i] = pixel
			pos += entryBytes
		}
	}

	sourceBytes := (h.pixelDepth + 7) / 8
	if sourceBytes <= 0 {
		return nil, errInvalidTGA
	}
	// Dimensions and output bytes were checked before palette parsing or
	// image allocation so compact RLE headers cannot reserve huge buffers.
	remaining := len(data) - pos
	if (!rle && pixelCount > remaining/sourceBytes) || (rle && pixelCount > (remaining/(sourceBytes+1)+1)*128) {
		return nil, errInvalidTGA
	}

	img := image.NewNRGBA(image.Rect(0, 0, h.width, h.height))
	writePixel := func(index int, pixel color.NRGBA) {
		x := index % h.width
		y := index / h.width
		if h.descriptor&0x10 != 0 {
			x = h.width - 1 - x
		}
		if h.descriptor&0x20 == 0 {
			y = h.height - 1 - y
		}
		offset := y*img.Stride + x*4
		img.Pix[offset+0] = pixel.R
		img.Pix[offset+1] = pixel.G
		img.Pix[offset+2] = pixel.B
		img.Pix[offset+3] = pixel.A
	}
	readPixel := func() (color.NRGBA, bool) {
		if pos > len(data)-sourceBytes {
			return color.NRGBA{}, false
		}
		raw := data[pos : pos+sourceBytes]
		pos += sourceBytes
		switch baseType {
		case 1:
			var index int
			if h.pixelDepth == 8 {
				index = int(raw[0])
			} else if h.pixelDepth == 16 {
				index = int(binary.LittleEndian.Uint16(raw))
			} else {
				return color.NRGBA{}, false
			}
			index -= h.colorMapFirst
			if index < 0 || index >= len(palette) {
				return color.NRGBA{}, false
			}
			return palette[index], true
		case 2:
			return decodeTGAColor(raw, h.pixelDepth, h.descriptor&0x0f != 0 || h.pixelDepth == 32)
		case 3:
			if h.pixelDepth == 8 {
				return color.NRGBA{R: raw[0], G: raw[0], B: raw[0], A: 0xff}, true
			}
			if h.pixelDepth == 16 {
				return color.NRGBA{R: raw[0], G: raw[0], B: raw[0], A: raw[1]}, true
			}
		}
		return color.NRGBA{}, false
	}

	for output := 0; output < pixelCount; {
		count := 1
		repeat := false
		if rle {
			if pos >= len(data) {
				return nil, errInvalidTGA
			}
			packet := data[pos]
			pos++
			count = int(packet&0x7f) + 1
			repeat = packet&0x80 != 0
			if count > pixelCount-output {
				return nil, errInvalidTGA
			}
		}
		if repeat {
			pixel, ok := readPixel()
			if !ok {
				return nil, errInvalidTGA
			}
			for range count {
				writePixel(output, pixel)
				output++
			}
			continue
		}
		for range count {
			pixel, ok := readPixel()
			if !ok {
				return nil, errInvalidTGA
			}
			writePixel(output, pixel)
			output++
		}
	}
	return img, nil
}

func decodeTGAColor(raw []byte, depth int, hasAlpha bool) (color.NRGBA, bool) {
	switch depth {
	case 15, 16:
		if len(raw) < 2 {
			return color.NRGBA{}, false
		}
		value := binary.LittleEndian.Uint16(raw)
		expand := func(v uint16) uint8 { return uint8((v << 3) | (v >> 2)) }
		alpha := uint8(0xff)
		if hasAlpha && value&0x8000 == 0 {
			alpha = 0
		}
		return color.NRGBA{
			R: expand((value >> 10) & 0x1f),
			G: expand((value >> 5) & 0x1f),
			B: expand(value & 0x1f),
			A: alpha,
		}, true
	case 24:
		if len(raw) < 3 {
			return color.NRGBA{}, false
		}
		return color.NRGBA{R: raw[2], G: raw[1], B: raw[0], A: 0xff}, true
	case 32:
		if len(raw) < 4 {
			return color.NRGBA{}, false
		}
		alpha := raw[3]
		if !hasAlpha {
			alpha = 0xff
		}
		return color.NRGBA{R: raw[2], G: raw[1], B: raw[0], A: alpha}, true
	}
	return color.NRGBA{}, false
}
