// Copyright 2019-2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fb

import (
	"fmt"
	"github.com/u-root/u-root/pkg/fb/fbdevice"
	"github.com/u-root/u-root/pkg/fb/fbimage"
	xdraw "golang.org/x/image/draw"
	"image"
	"image/color"
	"image/draw"
	"os"

	"github.com/orangecms/go-framebuffer/framebuffer"
)

const fbdev = "/dev/fb0"

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
			// framebuffer is BGR(A)
			buf[offset+0] = byte(b)
			buf[offset+1] = byte(g)
			buf[offset+2] = byte(r)
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

// copyRGBAtoBGR565 is an inlined version of the hot pixel copying loop for the
// special case of copying from an *image.RGBA to an *fbimage.BGR565.
//
// This specialization brings down copying time to 137ms (from 1.8s!) on the
// Raspberry Pi 4.
func copyRGBAtoBGR565(dst *fbimage.BGR565, src *image.RGBA) {
	bounds := dst.Bounds()
	for y := 0; y < bounds.Max.Y; y++ {
		for x := 0; x < bounds.Max.X; x++ {
			var c color.NRGBA

			i := src.PixOffset(x, y)
			// Small cap improves performance, see https://golang.org/issue/27857
			s := src.Pix[i : i+4 : i+4]
			switch s[3] {
			case 0xff:
				c = color.NRGBA{s[0], s[1], s[2], 0xff}
			case 0:
				c = color.NRGBA{0, 0, 0, 0}
			default:
				r := uint32(s[0])
				r |= r << 8
				g := uint32(s[1])
				g |= g << 8
				b := uint32(s[2])
				b |= b << 8
				a := uint32(s[3])
				a |= a << 8

				// Since Color.RGBA returns an alpha-premultiplied color, we
				// should have r <= a && g <= a && b <= a.
				r = (r * 0xffff) / a
				g = (g * 0xffff) / a
				b = (b * 0xffff) / a
				c = color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			}

			pix := dst.Pix[dst.PixOffset(x, y):]
			pix[0] = (c.B >> 3) | ((c.G >> 2) << 5)
			pix[1] = (c.G >> 5) | ((c.R >> 3) << 3)
		}
	}
}

func DrawImageAt(img image.Image, posx int, posy int) error {
	dev, err := fbdevice.Open(fbdev)

	devimg, err := dev.Image()
	if err != nil {
		return err
	}
	bounds := devimg.Bounds()

	// We do all rendering into an *image.RGBA buffer, for which all drawing
	// operations are optimized in Go. Only at the very end do we copy the
	// buffer contents to the (BGR565) framebuffer.
	buffer := image.NewRGBA(bounds)
	bgcolor := color.RGBA{R: 0x4f, G: 0x51, B: 0xa9, A: 255} // OSFC color #4f51a9

	draw.Draw(buffer, bounds, &image.Uniform{bgcolor}, image.Point{}, draw.Src)
	rect := scaleImage(img.Bounds(), bounds.Max.X, bounds.Max.Y)
	rect = rect.Add(image.Point{posx, posy})

	xdraw.BiLinear.Scale(buffer, rect, img, img.Bounds(), draw.Over, nil)

	// NOTE: This code path is NOT using double buffering (which is done
	// using the pan ioctl when using the frame buffer), but in practice
	// updates seem smooth enough, most likely because we are only
	// updating timestamps.
	if d, ok := devimg.(*fbimage.BGR565); ok {
		copyRGBAtoBGR565(d, buffer)
	} else {
		fmt.Printf("framebuffer not using pixel format BGR565")
	}
	draw.Draw(devimg, bounds, buffer, image.Point{}, draw.Src)
	/*
		err = os.WriteFile(fbdev, buf, 0o600)
		if err != nil {
			return fmt.Errorf("Error writing to framebuffer: %v", err)
		}
	*/
	return nil
}

func scaleImage(bounds image.Rectangle, maxW, maxH int) image.Rectangle {
	imgW := bounds.Max.X
	imgH := bounds.Max.Y
	ratio := float64(maxW) / float64(imgW)
	if r := float64(maxH) / float64(imgH); r < ratio {
		ratio = r
	}
	scaledW := int(ratio * float64(imgW))
	scaledH := int(ratio * float64(imgH))
	return image.Rect(0, 0, scaledW, scaledH)
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
	}
	buf := make([]byte, width*height*bpp)
	DrawScaledOnBufAt(buf, img, posx, posy, factor, stride, bpp)
	err = os.WriteFile(fbdev, buf, 0o600)
	if err != nil {
		return fmt.Errorf("Error writing to framebuffer: %v", err)
	}
	return nil
}
