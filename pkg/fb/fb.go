// Copyright 2019-2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fb

import (
	"fmt"
	"image"
	"io/ioutil"
	"os"

	"github.com/orangecms/go-framebuffer/framebuffer"
)

const fbdev = "/dev/fb0"

/*
	// YUV conversion
	var y float64 = 0.257*float64(r) + 0.504*float64(g) + 0.098*float64(b) + 16
	var u float64 = -0.148*float64(r) - 0.291*float64(g) + 0.439*float64(b) + 128
	var v float64 = 0.439*float64(r) - 0.368*float64(g) - 0.071*float64(b) + 128
	// BGR 565 conversion
	bgr := b >> 3
	bgr |= (r & 0xFC) << 3
	bgr |= (g & 0xF8) << 8
*/

func DrawOnBufAt(
	buf []byte,
	img image.Image,
	posx int,
	posy int,
	stride int,
	bpp int,
) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			offset := bpp * ((posy+y)*stride + posx + x)
			// 16-bit true color
			if bpp == 2 {
				// drop 3 lowest bits for each channel
				bgr := (b & 0xF8) >> 3
				bgr |= (g & 0xF8) << 2
				bgr |= (r & 0xF8) << 7
				// swap bytes through mask and shift
				buf[offset+0] = byte(bgr & 0xFF)
				// low byte, first bit (high bit) discarded
				buf[offset+1] = byte(bgr >> 8 & 0x7F)
				// framebuffer is BGR(A)
			} else if bpp >= 3 {
				buf[offset+0] = byte(b)
				buf[offset+1] = byte(g)
				buf[offset+2] = byte(r)
			}
			if bpp >= 4 {
				buf[offset+3] = byte(a)
			}
		}
	}
}

// FbInit initializes a frambuffer by querying ioctls and returns the width and
// height in pixels, the stride, and the bytes per pixel
func FbInit() (int, int, int, int, error) {
	fbo, err := framebuffer.Init(fbdev)

	if err != nil {
		return 0, 0, 0, 0, err
	}
	width, height := fbo.Size()
	stride := fbo.Stride()
	bpp := fbo.Bpp()
	fmt.Fprintf(os.Stdout, "Framebuffer resolution: %v %v %v %v\n", width, height, stride, bpp)
	return width, height, stride, bpp, nil
}

func DrawRainbowRectAt(
	buf []byte,
	posx int,
	posy int,
	stride int,
	bpp int,
) {
	offset := 0
	for y := 0; y < 256; y++ {
		for x := 0; x < 127; x++ {
			offset = (1*posx+x)*2 + (y+posy)*bpp*stride
			buf[offset+0] = byte(y)
			buf[offset+1] = byte(x)
		}
	}
}

func DrawPaletteAt(
	buf []byte,
	posx int,
	posy int,
	stride int,
	bpp int,
) {
	offset := 0
	for r := 0; r < 32; r++ {
		offset = (1*posx+r)*2 + (0+posy)*bpp*stride
		buf[offset+0] = byte(0)
		buf[offset+1] = byte(r << 2)
		offset = (1*posx+r)*2 + (1+posy)*bpp*stride
		buf[offset+0] = byte(0)
		buf[offset+1] = byte(r << 2)
		offset = (1*posx+r)*2 + (2+posy)*bpp*stride
		buf[offset+0] = byte(0)
		buf[offset+1] = byte(r << 2)
	}
	for b := 0; b < 32; b++ {
		offset = (1*posx+b)*2 + (3+posy)*bpp*stride
		buf[offset+0] = byte(b)
		buf[offset+1] = byte(0)
		offset = (1*posx+b)*2 + (4+posy)*bpp*stride
		buf[offset+0] = byte(b)
		buf[offset+1] = byte(0)
		offset = (1*posx+b)*2 + (5+posy)*bpp*stride
		buf[offset+0] = byte(b)
		buf[offset+1] = byte(0)
	}
	for g := 0; g < 32; g++ {
		gs := g << 5
		offset = (1*posx+g)*2 + (6+posy)*bpp*stride
		buf[offset+0] = byte(gs & 0xFF)
		buf[offset+1] = byte(gs >> 8 & 0x7F)
		offset = (1*posx+g)*2 + (7+posy)*bpp*stride
		buf[offset+0] = byte(gs & 0xFF)
		buf[offset+1] = byte(gs >> 8 & 0x7F)
		offset = (1*posx+g)*2 + (8+posy)*bpp*stride
		buf[offset+0] = byte(gs & 0xFF)
		buf[offset+1] = byte(gs >> 8 & 0x7F)
		offset = (1*posx+g)*2 + (9+posy)*bpp*stride
		buf[offset+0] = byte(gs & 0xFF)
		buf[offset+1] = byte(gs >> 8 & 0x7F)
	}
}

// NVR: 7372800 bytes fb
// 2 bytes per pixel, 3840 bytes per row
func DrawImageAt(img image.Image, posx int, posy int) error {
	width, height, stride, bpp, err := FbInit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Framebuffer init error: %v\n", err)
		// fallback values, 1080p 16bit
		// width, height, stride, bpp = 1920, 1080, 1920, 2
		width, height, stride, bpp = 1024, 768, 1024, 2
		fmt.Fprintf(os.Stdout, "Framebuffer fallback: %v %v %v %v\n", width, height, stride, bpp)
	}
	buf := make([]byte, stride*height*bpp)
	DrawOnBufAt(buf, img, posx, posy, stride, bpp)
	DrawRainbowRectAt(buf, 1780, 60, stride, bpp)
	DrawPaletteAt(buf, 1780, 360, stride, bpp)
	err = ioutil.WriteFile(fbdev, buf, 0600)
	if err != nil {
		return fmt.Errorf("Error writing to framebuffer: %v", err)
	}
	return nil
}

func DrawScaledOnBufAt(
	buf []byte,
	img image.Image,
	posx int,
	posy int,
	factor int,
	stride int,
	bpp int,
) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			for sx := 1; sx <= factor; sx++ {
				for sy := 1; sy <= factor; sy++ {
					offset := bpp * ((posy+y*factor+sy)*stride + posx + x*factor + sx)
					buf[offset+0] = byte(b)
					buf[offset+1] = byte(g)
					buf[offset+2] = byte(r)
					if bpp == 4 {
						buf[offset+3] = byte(a)
					}
				}
			}
		}
	}
}

func DrawScaledImageAt(img image.Image, posx int, posy int, factor int) error {
	width, height, stride, bpp, err := FbInit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Framebuffer init error: %v\n", err)
		// fallback values, 1080p 16bit
		width, height, stride, bpp = 1920, 1080, 1920, 2
		fmt.Fprintf(os.Stdout, "Framebuffer fallback: %v %v %v %v\n", width, height, stride, bpp)
	}
	buf := make([]byte, stride*height*bpp)
	DrawScaledOnBufAt(buf, img, posx, posy, factor, stride, bpp)
	err = ioutil.WriteFile(fbdev, buf, 0600)
	if err != nil {
		return fmt.Errorf("Error writing to framebuffer: %v", err)
	}
	return nil
}
