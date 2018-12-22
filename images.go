package main

import (
	"errors"
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
)

func decodeImage(r io.ReadSeeker) ([]image.Image, []time.Duration, error) {
	// is there a better way to determine type aside from reading the config?
	_, typ, err := image.DecodeConfig(r)
	if err != nil {
		return nil, nil, err
	}

	// seek back to start to re-read the whole image
	_, err = r.Seek(0, 0)
	if err != nil {
		return nil, nil, err
	}

	switch typ {
	case "gif":
		g, err := gif.DecodeAll(r)
		if err != nil {
			return nil, nil, err
		}
		var delays []time.Duration
		if g.Delay != nil {
			delays = make([]time.Duration, len(g.Delay))
			for i, d := range g.Delay {
				delays[i] = time.Duration(d) * time.Second / 100
			}
		}
		return makeImageArray(g), delays, errors.New("not implemented")
	default:
		im, _, err := image.Decode(r)
		if err != nil {
			return nil, nil, err
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
			drawFrame(idx, frame, canvas, frames[len(frames)-1], img, g)
		} else {
			drawFrame(idx, frame, canvas, nil, img, g)
		}

		frames = append(frames, frame)
	}

	if len(frames) > 1 {
		// redo first frame
		frame = image.NewRGBA(canvas.Bounds())
		canvas = image.NewRGBA(frame.Bounds())
		drawFrame(0, frame, canvas, frames[len(frames)-1], g.Image[0], g)
		frames[0] = frame

	}

	return frames
}

func drawFrame(idx int, frame, canvas *image.RGBA, prev image.Image, img *image.Paletted, g *gif.GIF) {
	// draw previous frame blanked
	if prev != nil {
		draw.DrawMask(frame, frame.Bounds(), image.NewUniform(color.RGBA{0, 0, 0, 255}), image.Point{0, 0}, maskNonTransparent{prev}, image.Point{0, 0}, draw.Src)
	}

	switch g.Disposal[idx] {
	// untested
	case gif.DisposalPrevious:
		prev := copyRGBA(canvas.SubImage(img.Bounds()).(*image.RGBA))
		draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Over)
		draw.Draw(frame, frame.Bounds(), canvas, image.Point{0, 0}, draw.Over)
		draw.Draw(canvas, img.Bounds(), prev, image.Point{0, 0}, draw.Src)
	case gif.DisposalBackground:
		draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Over)
		draw.Draw(frame, frame.Bounds(), canvas, image.Point{0, 0}, draw.Over)
		draw.Draw(canvas, img.Bounds(), image.NewUniform(img.Palette[g.BackgroundIndex]), img.Bounds().Min, draw.Src)
	default:
		draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Over)
		draw.Draw(frame, frame.Bounds(), canvas, image.Point{0, 0}, draw.Over)
	}
}

func copyRGBA(im *image.RGBA) *image.RGBA {
	second := image.NewRGBA(im.Bounds())
	draw.Draw(second, im.Bounds(), im, image.Point{0, 0}, draw.Src)
	return second
}

type maskNonTransparent struct {
	image.Image
}

func (i maskNonTransparent) Bounds() image.Rectangle {
	return i.Image.Bounds()
}

func (i maskNonTransparent) ColorModel() color.Model {
	return color.AlphaModel
}

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
