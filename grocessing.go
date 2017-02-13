package grocessing

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/sdl_image"
	"github.com/veandco/go-sdl2/sdl_ttf"
)

var (
	// KeyCode contains current pressed key
	KeyCode Key

	// Mouse State
	MouseX, MouseY, PMouseX, PMouseY int
)

var (
	window   *sdl.Window
	renderer *sdl.Renderer
	font     *ttf.Font

	fpsCap    uint = 30
	frames    uint
	lastStart int64

	sketch Sketch

	stop = make(chan bool)

	winTitle = "Debug view"

	m     *Matrix
	stack []*Matrix
)

const (
	STYLE_NORMAL = iota
	STYLE_BOLD

	KEY_UP     = Key(sdl.K_UP)
	KEY_DOWN   = Key(sdl.K_DOWN)
	KEY_LEFT   = Key(sdl.K_LEFT)
	KEY_RIGHT  = Key(sdl.K_RIGHT)
	KEY_RETURN = Key(sdl.K_RETURN)
	KEY_ESC    = Key(sdl.K_ESCAPE)
	KEY_SPACE  = Key(sdl.K_SPACE)

	ALIGN_CENTER = iota
	ALIGN_LEFT
)

type Matrix struct {
	fillColor   Color
	strokeColor Color
	draw_stroke bool
	draw_fill   bool
	textStyle   int
	textAlign   int
	x, y        int
}

type Color *sdl.Color
type Font *ttf.Font

type Image struct {
	*sdl.Texture
	w, h int32
}

type Sketch interface {
	Setup()
	Draw()
	KeyPressed()
}

type Key uint32

func GrocessingStart(s Sketch) {

	sketch = s
	err := ttf.Init()
	if err != nil {
		panic(err)
	}

	m = default_matrix()
	stack = []*Matrix{m}

	window, err = sdl.CreateWindow(winTitle, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		0, 0, sdl.WINDOW_SHOWN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create window: %s\n", err)
		os.Exit(1)
	}
	defer window.Destroy()

	renderer, err = sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create renderer: %s\n", err)
		os.Exit(2)
	}

	sketch.Setup()

	//draw routine
	go func() {
		defer renderer.Destroy()
		runtime.LockOSThread()
		var diff int64
		for {
			lastStart := time.Now().UnixNano()
			renderer.Clear()
			sketch.Draw()
			renderer.Present()
			diff += time.Now().UnixNano() - lastStart
			if diff >= 1000 {
				frames = 0
			}
			frames++

			go checkEvent()

			if frames < 1000/fpsCap {
				sdl.Delay(uint32(1000/fpsCap - frames))
			}
		}
	}()

	<-stop
	os.Exit(0)
}

func checkEvent() {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch t := event.(type) {
		case *sdl.QuitEvent:
			stop <- true
		case *sdl.KeyDownEvent:
			KeyCode = Key(t.Keysym.Sym)
			sketch.KeyPressed()
		case *sdl.MouseMotionEvent:
			PMouseX, PMouseY = MouseX, MouseY
			MouseX, MouseY = int(t.X), int(t.Y)
		}
	}
}

func default_matrix() *Matrix {
	return &Matrix{
		fillColor:   Hc(0),
		strokeColor: Hc(0xffffff),
		draw_stroke: true,
		draw_fill:   true,
	}
}

func Rgb(r, g, b byte) Color {
	return &sdl.Color{r, g, b, 0}
}

func Hc(h int32) Color {
	return &sdl.Color{
		uint8((h >> 16) & 0xFF),
		uint8((h >> 8) & 0xFF),
		uint8((h >> 0) & 0xFF),
		0,
	}
}

func Title(title string) {
	window.SetTitle(title)
}

func NoFill() {
	m.draw_fill = false
}

func CreateFont(name string, a int) (Font, error) {
	font, err := ttf.OpenFont(name, a)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not open font: %v", err))
	}
	return font, nil
}

func Size(w, h int) {
	window.SetSize(w, h)
}

func SetFont(f *ttf.Font) {
	font = f
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func Fill(c Color) {
	m.draw_fill = true
	m.fillColor = c
}

func Stroke(c Color) {
	m.strokeColor = c
}

func Background(c Color) {
	Fill(c)
	m.draw_fill = true
	w, h := window.GetSize()
	Rect(0, 0, w, h)
}

func Rect(x, y, w, h int) {
	r := &sdl.Rect{
		int32(x + m.x),
		int32(y + m.y),
		int32(w),
		int32(h),
	}

	if m.draw_fill && m.fillColor != nil {
		renderer.SetDrawColor(
			m.fillColor.R, m.fillColor.G, m.fillColor.B, m.fillColor.A,
		)
		renderer.FillRect(r)
	}

	if m.draw_stroke && m.strokeColor != nil {
		renderer.SetDrawColor(
			m.strokeColor.R, m.strokeColor.G, m.strokeColor.B, m.strokeColor.A,
		)
		renderer.DrawRect(r)
	}

}

func TextAlign(a int) {
	m.textAlign = a
}

func Text(txt string, x, y, w, h int) {
	var (
		surface *sdl.Surface
		err     error
	)
	if len(txt) == 0 {
		return
	}
	switch m.textStyle {
	case STYLE_NORMAL:
		surface, err = font.RenderUTF8_Blended(txt, *m.fillColor)
	case STYLE_BOLD:
		surface, err = font.RenderUTF8_Solid(txt, *m.fillColor)
	}
	if err != nil {
		panic(err)
	}
	texture, err := renderer.CreateTextureFromSurface(surface)
	surface.Free()
	if err != nil {
		panic(err)
	}

	rw, rh, err := font.SizeUTF8(txt)
	var r sdl.Rect

	switch m.textAlign {
	case ALIGN_CENTER:
		r = sdl.Rect{
			int32(m.x + x + (Max(rw, int(w))-Min(rw, int(w)))/2),
			int32(m.y + y + (Max(rh, int(h))-Min(rh, int(h)))/2),
			int32(rw),
			int32(rh),
		}
	case ALIGN_LEFT:
		r = sdl.Rect{
			int32(m.x + x),
			int32(m.y + y),
			int32(rw),
			int32(rh),
		}

	}

	renderer.Copy(texture, nil, &r)
	texture.Destroy()
}

func TextStyle(style int) {
	m.textStyle = style
}

func LoadImage(path string) (*Image, error) {

	image, err := img.Load(path)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not load image: %v", err))
	}

	texture, err := renderer.CreateTextureFromSurface(image)
	image.Free()

	_, _, w, h, err := texture.Query()
	if err != nil {
		return nil, err
	}

	return &Image{texture, w, h}, nil
}

func (i *Image) Draw(x, y int) {
	i.DrawRect(int(x), int(y), int(i.w), int(i.h))
}

func (i *Image) DrawRect(x, y, w, h int) {
	renderer.Copy(i.Texture,
		&sdl.Rect{
			0, 0,
			i.w, i.h,
		},
		&sdl.Rect{
			int32(x + m.x), int32(y + m.y),
			int32(w), int32(h),
		})
}

func (i *Image) Free() {
	i.Texture.Destroy()
}

func PushMatrix() {
	new_mat := *m
	stack = append(stack, m)
	m = &new_mat
}

func PopMatrix() {
	m = stack[len(stack)-1]
	stack = stack[:len(stack)-1]
}

func Translate(x, y int) {
	m.x += x
	m.y += y
}
