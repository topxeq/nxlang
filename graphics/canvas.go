// Package graphics provides image processing and graphics capabilities for Nxlang
package graphics

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"github.com/topxeq/nxlang/types"
	"github.com/topxeq/nxlang/types/collections"
)

// Canvas represents a drawing canvas for graphics operations
type Canvas struct {
	img    *image.RGBA
	width  int
	height int
}

// NewCanvas creates a new canvas with specified dimensions
func NewCanvas(width, height int) *Canvas {
	return &Canvas{
		img:    image.NewRGBA(image.Rect(0, 0, width, height)),
		width:  width,
		height: height,
	}
}

// TypeCode implements types.Object interface
func (c *Canvas) TypeCode() uint8 {
	return 0x50 // Canvas type code
}

// TypeName implements types.Object interface
func (c *Canvas) TypeName() string {
	return "canvas"
}

// ToStr implements types.Object interface
func (c *Canvas) ToStr() string {
	return "Canvas[" + itoa(c.width) + "x" + itoa(c.height) + "]"
}

// Equals implements types.Object interface
func (c *Canvas) Equals(other types.Object) bool {
	otherCanvas, ok := other.(*Canvas)
	if !ok {
		return false
	}
	return c == otherCanvas
}

// Width returns the canvas width
func (c *Canvas) Width() int {
	return c.width
}

// Height returns the canvas height
func (c *Canvas) Height() int {
	return c.height
}

// Clear clears the canvas with the specified color
func (c *Canvas) Clear(r, g, b, a uint8) {
	draw.Draw(c.img, c.img.Bounds(), &image.Uniform{color.RGBA{r, g, b, a}}, image.Point{}, draw.Src)
}

// DrawPoint draws a single point on the canvas
func (c *Canvas) DrawPoint(x, y int, r, g, b, a uint8) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		c.img.SetRGBA(x, y, color.RGBA{r, g, b, a})
	}
}

// DrawLine draws a line using Bresenham's algorithm
func (c *Canvas) DrawLine(x0, y0, x1, y1 int, r, g, b, a uint8) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx - dy

	for {
		c.DrawPoint(x0, y0, r, g, b, a)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// DrawRectangle draws a rectangle outline
func (c *Canvas) DrawRectangle(x, y, w, h int, r, g, b, a uint8) {
	c.DrawLine(x, y, x+w, y, r, g, b, a)       // Top
	c.DrawLine(x, y+h, x+w, y+h, r, g, b, a)   // Bottom
	c.DrawLine(x, y, x, y+h, r, g, b, a)       // Left
	c.DrawLine(x+w, y, x+w, y+h, r, g, b, a)   // Right
}

// FillRectangle draws a filled rectangle
func (c *Canvas) FillRectangle(x, y, w, h int, r, g, b, a uint8) {
	for i := 0; i < w; i++ {
		for j := 0; j < h; j++ {
			c.DrawPoint(x+i, y+j, r, g, b, a)
		}
	}
}

// DrawCircle draws a circle outline using midpoint circle algorithm
func (c *Canvas) DrawCircle(cx, cy, radius int, r, g, b, a uint8) {
	x := radius
	y := 0
	err := 0

	for x >= y {
		c.DrawPoint(cx+x, cy+y, r, g, b, a)
		c.DrawPoint(cx+y, cy+x, r, g, b, a)
		c.DrawPoint(cx-y, cy+x, r, g, b, a)
		c.DrawPoint(cx-x, cy+y, r, g, b, a)
		c.DrawPoint(cx-x, cy-y, r, g, b, a)
		c.DrawPoint(cx-y, cy-x, r, g, b, a)
		c.DrawPoint(cx+y, cy-x, r, g, b, a)
		c.DrawPoint(cx+x, cy-y, r, g, b, a)

		if err <= 0 {
			y++
			err += 2*y + 1
		}
		if err > 0 {
			x--
			err -= 2*x + 1
		}
	}
}

// FillCircle draws a filled circle
func (c *Canvas) FillCircle(cx, cy, radius int, r, g, b, a uint8) {
	for x := -radius; x <= radius; x++ {
		for y := -radius; y <= radius; y++ {
			if x*x+y*y <= radius*radius {
				c.DrawPoint(cx+x, cy+y, r, g, b, a)
			}
		}
	}
}

// GetPixel returns the color of a pixel
func (c *Canvas) GetPixel(x, y int) (uint8, uint8, uint8, uint8) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return 0, 0, 0, 0
	}
	rgba := c.img.RGBAAt(x, y)
	return rgba.R, rgba.G, rgba.B, rgba.A
}

// SaveToPNG saves the canvas to a PNG file
func (c *Canvas) SaveToPNG(filename string) types.Object {
	file, err := os.Create(filename)
	if err != nil {
		return types.NewError("failed to create file: "+err.Error(), 0, 0, "")
	}
	defer file.Close()

	if err := png.Encode(file, c.img); err != nil {
		return types.NewError("failed to encode PNG: "+err.Error(), 0, 0, "")
	}

	return types.Bool(true)
}

// LoadFromPNG loads an image from a PNG file
func (c *Canvas) LoadFromPNG(filename string) types.Object {
	file, err := os.Open(filename)
	if err != nil {
		return types.NewError("failed to open file: "+err.Error(), 0, 0, "")
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return types.NewError("failed to decode PNG: "+err.Error(), 0, 0, "")
	}

	bounds := img.Bounds()
	c.width = bounds.Dx()
	c.height = bounds.Dy()
	c.img = image.NewRGBA(bounds)
	draw.Draw(c.img, bounds, img, bounds.Min, draw.Src)

	return types.Bool(true)
}

// ToBytes returns the raw RGBA bytes of the canvas
func (c *Canvas) ToBytes() []byte {
	return c.img.Pix
}

// FromBytes loads image data from raw RGBA bytes
func (c *Canvas) FromBytes(data []byte, width, height int) types.Object {
	if len(data) != width*height*4 {
		return types.NewError("invalid data size", 0, 0, "")
	}

	c.width = width
	c.height = height
	c.img = &image.RGBA{
		Pix:    data,
		Stride: 4 * width,
		Rect:   image.Rect(0, 0, width, height),
	}

	return types.Bool(true)
}

// Helper functions
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [32]byte
	j := len(buf)
	for i > 0 {
		j--
		buf[j] = byte(i%10 + '0')
		i /= 10
	}
	if neg {
		j--
		buf[j] = '-'
	}
	return string(buf[j:])
}

// Image represents a loaded image
type Image struct {
	img    image.Image
	width  int
	height int
}

// TypeCode implements types.Object interface
func (i *Image) TypeCode() uint8 {
	return 0x51 // Image type code
}

// TypeName implements types.Object interface
func (i *Image) TypeName() string {
	return "image"
}

// ToStr implements types.Object interface
func (i *Image) ToStr() string {
	return "Image[" + itoa(i.width) + "x" + itoa(i.height) + "]"
}

// Equals implements types.Object interface
func (i *Image) Equals(other types.Object) bool {
	otherImage, ok := other.(*Image)
	if !ok {
		return false
	}
	return i == otherImage
}

// Width returns the image width
func (i *Image) Width() int {
	return i.width
}

// Height returns the image height
func (i *Image) Height() int {
	return i.height
}

// LoadImage loads an image from a file (supports PNG, JPEG, GIF)
func LoadImage(filename string) (*Image, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var img image.Image
	lowerFilename := strings.ToLower(filename)
	if strings.HasSuffix(lowerFilename, ".png") {
		img, err = png.Decode(file)
	} else {
		// For simplicity, only PNG is fully supported in this version
		return nil, os.ErrNotExist
	}

	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	return &Image{
		img:    img,
		width:  bounds.Dx(),
		height: bounds.Dy(),
	}, nil
}

// ToCanvas converts the image to a canvas
func (i *Image) ToCanvas() *Canvas {
	bounds := i.img.Bounds()
	canvas := NewCanvas(bounds.Dx(), bounds.Dy())
	draw.Draw(canvas.img, bounds, i.img, bounds.Min, draw.Src)
	return canvas
}

// Graphics functions for Nxlang

// CreateCanvasFunc creates a new canvas
func CreateCanvasFunc(args ...types.Object) types.Object {
	if len(args) < 2 {
		return types.NewError("canvas() expects 2 arguments (width, height)", 0, 0, "")
	}
	width, err := types.ToInt(args[0])
	if err != nil {
		return err
	}
	height, err := types.ToInt(args[1])
	if err != nil {
		return err
	}
	return NewCanvas(int(width), int(height))
}

// ClearFunc clears a canvas
func ClearFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("clear() expects at least 1 argument (canvas)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	r, g, b, a := uint8(0), uint8(0), uint8(0), uint8(255)
	if len(args) >= 5 {
		r64, _ := types.ToInt(args[1])
		g64, _ := types.ToInt(args[2])
		b64, _ := types.ToInt(args[3])
		a64, _ := types.ToInt(args[4])
		r, g, b, a = uint8(r64), uint8(g64), uint8(b64), uint8(a64)
	}
	canvas.Clear(r, g, b, a)
	return args[0]
}

// DrawPointFunc draws a point on canvas
func DrawPointFunc(args ...types.Object) types.Object {
	if len(args) < 4 {
		return types.NewError("drawPoint() expects at least 4 arguments (canvas, x, y, color)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	x64, err := types.ToInt(args[1])
	if err != nil {
		return err
	}
	y64, err := types.ToInt(args[2])
	if err != nil {
		return err
	}
	r64, _ := types.ToInt(args[3])
	g64, _ := types.ToInt(args[4])
	b64, _ := types.ToInt(args[5])
	a64, _ := types.ToInt(args[6])
	canvas.DrawPoint(int(x64), int(y64), uint8(r64), uint8(g64), uint8(b64), uint8(a64))
	return args[0]
}

// DrawLineFunc draws a line on canvas
func DrawLineFunc(args ...types.Object) types.Object {
	if len(args) < 6 {
		return types.NewError("drawLine() expects at least 6 arguments (canvas, x0, y0, x1, y1, color)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	x064, _ := types.ToInt(args[1])
	y064, _ := types.ToInt(args[2])
	x164, _ := types.ToInt(args[3])
	y164, _ := types.ToInt(args[4])
	r64, _ := types.ToInt(args[5])
	g64, _ := types.ToInt(args[6])
	b64, _ := types.ToInt(args[7])
	a64, _ := types.ToInt(args[8])
	canvas.DrawLine(int(x064), int(y064), int(x164), int(y164), uint8(r64), uint8(g64), uint8(b64), uint8(a64))
	return args[0]
}

// DrawRectangleFunc draws a rectangle on canvas
func DrawRectangleFunc(args ...types.Object) types.Object {
	if len(args) < 5 {
		return types.NewError("drawRectangle() expects at least 5 arguments (canvas, x, y, w, h, color)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	x64, _ := types.ToInt(args[1])
	y64, _ := types.ToInt(args[2])
	w64, _ := types.ToInt(args[3])
	h64, _ := types.ToInt(args[4])
	r64, _ := types.ToInt(args[5])
	g64, _ := types.ToInt(args[6])
	b64, _ := types.ToInt(args[7])
	a64, _ := types.ToInt(args[8])
	canvas.DrawRectangle(int(x64), int(y64), int(w64), int(h64), uint8(r64), uint8(g64), uint8(b64), uint8(a64))
	return args[0]
}

// FillRectangleFunc fills a rectangle on canvas
func FillRectangleFunc(args ...types.Object) types.Object {
	if len(args) < 5 {
		return types.NewError("fillRectangle() expects at least 5 arguments (canvas, x, y, w, h, color)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	x64, _ := types.ToInt(args[1])
	y64, _ := types.ToInt(args[2])
	w64, _ := types.ToInt(args[3])
	h64, _ := types.ToInt(args[4])
	r64, _ := types.ToInt(args[5])
	g64, _ := types.ToInt(args[6])
	b64, _ := types.ToInt(args[7])
	a64, _ := types.ToInt(args[8])
	canvas.FillRectangle(int(x64), int(y64), int(w64), int(h64), uint8(r64), uint8(g64), uint8(b64), uint8(a64))
	return args[0]
}

// DrawCircleFunc draws a circle on canvas
func DrawCircleFunc(args ...types.Object) types.Object {
	if len(args) < 4 {
		return types.NewError("drawCircle() expects at least 4 arguments (canvas, cx, cy, radius, color)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	cx64, _ := types.ToInt(args[1])
	cy64, _ := types.ToInt(args[2])
	r64, _ := types.ToInt(args[3])
	r64Color, _ := types.ToInt(args[4])
	g64, _ := types.ToInt(args[5])
	b64, _ := types.ToInt(args[6])
	a64, _ := types.ToInt(args[7])
	canvas.DrawCircle(int(cx64), int(cy64), int(r64), uint8(r64Color), uint8(g64), uint8(b64), uint8(a64))
	return args[0]
}

// FillCircleFunc fills a circle on canvas
func FillCircleFunc(args ...types.Object) types.Object {
	if len(args) < 4 {
		return types.NewError("fillCircle() expects at least 4 arguments (canvas, cx, cy, radius, color)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	cx64, _ := types.ToInt(args[1])
	cy64, _ := types.ToInt(args[2])
	r64, _ := types.ToInt(args[3])
	r64Color, _ := types.ToInt(args[4])
	g64, _ := types.ToInt(args[5])
	b64, _ := types.ToInt(args[6])
	a64, _ := types.ToInt(args[7])
	canvas.FillCircle(int(cx64), int(cy64), int(r64), uint8(r64Color), uint8(g64), uint8(b64), uint8(a64))
	return args[0]
}

// SavePNGFunc saves canvas to PNG file
func SavePNGFunc(args ...types.Object) types.Object {
	if len(args) < 2 {
		return types.NewError("savePNG() expects 2 arguments (canvas, filename)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	filename := string(types.ToString(args[1]))
	return canvas.SaveToPNG(filename)
}

// GetPixelFunc gets pixel color from canvas
func GetPixelFunc(args ...types.Object) types.Object {
	if len(args) < 3 {
		return types.NewError("getPixel() expects 3 arguments (canvas, x, y)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("first argument must be a canvas", 0, 0, "")
	}
	x64, err := types.ToInt(args[1])
	if err != nil {
		return err
	}
	y64, err := types.ToInt(args[2])
	if err != nil {
		return err
	}
	r, g, b, a := canvas.GetPixel(int(x64), int(y64))
	result := collections.NewArray()
	result.Append(types.Int(r))
	result.Append(types.Int(g))
	result.Append(types.Int(b))
	result.Append(types.Int(a))
	return result
}

// CanvasGetWidth returns canvas width
func CanvasGetWidth(args ...types.Object) types.Object {
	if len(args) != 1 {
		return types.NewError("canvasWidth() expects 1 argument (canvas)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("argument must be a canvas", 0, 0, "")
	}
	return types.Int(canvas.Width())
}

// CanvasGetHeight returns canvas height
func CanvasGetHeight(args ...types.Object) types.Object {
	if len(args) != 1 {
		return types.NewError("canvasHeight() expects 1 argument (canvas)", 0, 0, "")
	}
	canvas, ok := args[0].(*Canvas)
	if !ok {
		return types.NewError("argument must be a canvas", 0, 0, "")
	}
	return types.Int(canvas.Height())
}

// LoadPNGFunc loads a PNG image into a canvas
func LoadPNGFunc(args ...types.Object) types.Object {
	if len(args) < 1 {
		return types.NewError("loadPNG() expects 1 argument (filename)", 0, 0, "")
	}
	filename := string(types.ToString(args[0]))
	canvas := NewCanvas(1, 1)
	result := canvas.LoadFromPNG(filename)
	if _, ok := result.(types.Object); ok && result.ToStr() != "true" {
		return result
	}
	return canvas
}
