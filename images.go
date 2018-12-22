package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path"
	"time"

	"github.com/pkg/errors"
)

// decodeImage reads in an image file, specifically handling the case where
// the image is an animated GIF. The first return value will always be a slice,
// but may contain just one image if the file read is not animated. The second
// return value
func decodeImage(r io.ReadSeeker) ([]image.Image, []time.Duration, error) {
	// is there a better way to determine type aside from reading the config?
	_, typ, err := image.DecodeConfig(r)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not decode image config")
	}

	// seek back to start to re-read the whole image
	_, err = r.Seek(0, 0)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not seek reader")
	}

	switch typ {
	case "gif":
		g, err := gif.DecodeAll(r)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not decode all GIF frames")
		}
		var delays []time.Duration
		if g.Delay != nil {
			delays = make([]time.Duration, len(g.Delay))
			for i, d := range g.Delay {
				delays[i] = time.Duration(d) * time.Second / 100
			}
		}
		return makeImageArray(g), delays, nil
	default:
		im, _, err := image.Decode(r)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not decode image")
		}
		return []image.Image{im}, nil, nil
	}
}

// this is a nightmare of a function to take a gif image and produce a sequence
// of images, such that pixels from the previous frame are appropriately blanked
// if they are not covered by a new pixel
func makeImageArray(g *gif.GIF) []image.Image {
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))

	var frame *image.RGBA
	var frames []image.Image
	for idx, img := range g.Image {
		frame = image.NewRGBA(canvas.Bounds())

		if len(frames) > 0 {
			drawFrame(canvas, frame, img, g.Disposal[idx], img.Palette[g.BackgroundIndex], frames[len(frames)-1])
		} else {
			drawFrame(canvas, frame, img, g.Disposal[idx], img.Palette[g.BackgroundIndex], nil)
		}

		frames = append(frames, frame)
	}

	if len(frames) > 1 {
		// redo first frame, so it looks good on loop
		frame = image.NewRGBA(canvas.Bounds())
		canvas = image.NewRGBA(frame.Bounds())
		drawFrame(canvas, frame, g.Image[0], gif.DisposalNone, nil, frames[len(frames)-1])
		frames[0] = frame
	}

	return frames
}

// drawFrame handles the heavy lifting of drawing a particular frame from an
// animated GIF. The individual frames of a GIF may contain only a partial
// update to the image, and depending on the disposal method, may be un-drawn
// in certain ways. This method handles drawing a particular frame from a GIF
// onto the display canvas and yields an image frame representing the
// combination of the canvas and the current GIF frame, i.e., the point-in-time
// state of the animation. It then restores the canvas acording to the frame
// disposal method. If prev is provided, the frame is pre-drawn with a black
// mask of prev's non-transparent pixels. This is used to ensure previously
// lighted pixels on the display board are blacked on the next frame.
func drawFrame(canvas, frame *image.RGBA, img *image.Paletted, disposal byte, background color.Color, prev image.Image) {
	// draw previous frame blanked
	if prev != nil {
		draw.DrawMask(frame, frame.Bounds(), image.NewUniform(color.RGBA{0, 0, 0, 255}), image.Point{0, 0}, maskNonTransparent{prev}, image.Point{0, 0}, draw.Src)
	}

	switch disposal {
	// untested
	case gif.DisposalPrevious:
		prev := copyRGBA(canvas.SubImage(img.Bounds()).(*image.RGBA))
		draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Over)
		draw.Draw(frame, frame.Bounds(), canvas, image.Point{0, 0}, draw.Over)
		draw.Draw(canvas, prev.Bounds(), prev, img.Bounds().Min, draw.Src)
	case gif.DisposalBackground:
		draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Over)
		draw.Draw(frame, frame.Bounds(), canvas, image.Point{0, 0}, draw.Over)
		draw.Draw(canvas, img.Bounds(), image.NewUniform(background), img.Bounds().Min, draw.Src)
	default:
		draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Over)
		draw.Draw(frame, frame.Bounds(), canvas, image.Point{0, 0}, draw.Over)
	}
}

// copyRGBA copies an RGBA image. It was supposed to be an optimization for
// copying the canvas for the current frame specific to RGBA, but a generic
// draw implementation was used instead for specifically typed images.
func copyRGBA(im *image.RGBA) *image.RGBA {
	second := image.NewRGBA(im.Bounds())
	draw.Draw(second, im.Bounds(), im, image.Point{0, 0}, draw.Src)
	return second
}

// maskNonTransparent wraps an existing image and is useful for generating an
// alpha mask of the non-black and non-transparent pixels of the image.
type maskNonTransparent struct {
	image.Image
}

// Bounds returns the bounds of the wrapped image, since the mask should be the
// same size.
func (i maskNonTransparent) Bounds() image.Rectangle {
	return i.Image.Bounds()
}

// ColorModel is always AlphaModel since we are generating an alpha mask.
func (i maskNonTransparent) ColorModel() color.Model {
	return color.AlphaModel
}

// At returns a fully opaque pixel if the wrapped image has a non-black and non-
// transparent pixel at the same location. It ignores black pixels since black
// pixels are "invisible" on the lighted display board.
func (i maskNonTransparent) At(x, y int) color.Color {
	r, g, b, a := i.Image.At(x, y).RGBA()
	// transparent or black pixel
	if a != 0 && r != 0 && g != 0 && b != 0 {
		return color.Alpha{255}
	}
	return color.Alpha{0}
}

// useful during testing to save the image canvas and the current frame
func save(idx int, frame, canvas image.Image) {
	f, err := os.Create(path.Join("frames", fmt.Sprintf("f%04d.png", idx)))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	err = png.Encode(f, frame)
	if err != nil {
		panic(err)
	}

	f, err = os.Create(path.Join("frames", fmt.Sprintf("c%04d.png", idx)))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	err = png.Encode(f, canvas)
	if err != nil {
		panic(err)
	}
}
