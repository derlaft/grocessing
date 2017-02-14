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

	// global sdl drawing stuff
	window   *sdl.Window
	renderer *sdl.Renderer
	font     *ttf.Font

	// fps control stuff
	fpsCap              int64 = 60
	fps                 uint
	lastStart           int64
	secondDuration      = time.Second.Nanoseconds()
	millisecondDuration = time.Millisecond.Nanoseconds()

	// main sketch
	sketch Sketch

	// channel to await termination
	stop    = make(chan bool)
	running = true

	winTitle = "Debug view"

	// drawing states stack (for pop/push use)
	m     *matrix
	stack []*matrix
)

const (
	// Font styles
	STYLE_NORMAL = iota
	STYLE_BOLD

	// Text aligment options
	ALIGN_CENTER = iota
	ALIGN_LEFT

	// Keyboard keys
	KEY_UP     = Key(sdl.K_UP)
	KEY_DOWN   = Key(sdl.K_DOWN)
	KEY_LEFT   = Key(sdl.K_LEFT)
	KEY_RIGHT  = Key(sdl.K_RIGHT)
	KEY_RETURN = Key(sdl.K_RETURN)
	KEY_ESC    = Key(sdl.K_ESCAPE)
	KEY_SPACE  = Key(sdl.K_SPACE)
)

// wrapper types
type (
	Color *sdl.Color
	Font  *ttf.Font
	Key   uint32

	Image struct {
		*sdl.Texture
		w, h int32
	}
)

// sketch one-method interfaces
type (
	Sketch interface {
		Draw()
	}

	SketchSetup interface {
		Setup()
	}

	SketchKeyPressed interface {
		KeyPressed()
	}

	SketchMouseClicked interface {
		MouseClicked()
	}
)

// defines drawing state
type matrix struct {
	fillColor   Color
	strokeColor Color
	draw_stroke bool
	draw_fill   bool
	textStyle   int
	textAlign   int
	x, y        int
}

// GrocessingStart is library entry-point
// I wish one could create 'main()' in the library
func GrocessingStart(s Sketch) {

	sketch = s
	err := ttf.Init()
	if err != nil {
		panic(err)
	}

	m = default_matrix()
	stack = []*matrix{m}

	window, err = sdl.CreateWindow(winTitle, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		0, 0, sdl.WINDOW_SHOWN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create window: %s\n", err)
		os.Exit(1)
	}
	defer window.Destroy()

	// That (in theory) should enable anti-alising
	// but I see no diffrence at all
	// @TODO: check if it works
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "1")

	renderer, err = sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create renderer: %s\n", err)
		os.Exit(2)
	}

	if ss, ok := sketch.(SketchSetup); ok {
		ss.Setup()
	}

	go mainLoop()

	<-stop
	os.Exit(0)
}

// draw routine
// @TODO: stop it
func mainLoop() {

	// it is important to lock main thread there
	defer renderer.Destroy()
	runtime.LockOSThread()

	frameNotify := make(chan bool)
	defer close(frameNotify)
	go measureFps(frameNotify)

	for running {
		go checkEvent()

		lastStart := time.Now().UnixNano()

		renderer.Clear()
		sketch.Draw()
		renderer.Present()
		frameNotify <- true

		diff := time.Now().UnixNano() - lastStart

		if diff < secondDuration/fpsCap {
			sdl.Delay(uint32((secondDuration/fpsCap - diff) / millisecondDuration))
		}
	}
}

func measureFps(newFrameDrawed <-chan bool) {
	var frames int64
	measurementStart := time.Now()

	for range newFrameDrawed {
		if frames++; frames == 100 {
			fps = uint(float64(frames) / time.Since(measurementStart).Seconds())
			frames = 0
			measurementStart = time.Now()
		}
	}

}

// event polling thread
func checkEvent() {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch t := event.(type) {
		case *sdl.QuitEvent:
			running = false
			stop <- true
		case *sdl.KeyDownEvent:
			KeyCode = Key(t.Keysym.Sym)
			if ss, ok := sketch.(SketchKeyPressed); ok {
				ss.KeyPressed()
			}
		case *sdl.MouseMotionEvent:
			PMouseX, PMouseY = MouseX, MouseY
			MouseX, MouseY = int(t.X), int(t.Y)
		case *sdl.MouseButtonEvent:
			if ss, ok := sketch.(SketchMouseClicked); ok {
				ss.MouseClicked()
			}
		}
	}
}

func default_matrix() *matrix {
	return &matrix{
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

func CreateFont(filename string, size int) (Font, error) {
	font, err := ttf.OpenFont(filename, size)
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

func Line(x1, y1, x2, y2 int) {
	if m.draw_stroke && m.strokeColor != nil {
		renderer.SetDrawColor(
			m.strokeColor.R, m.strokeColor.G, m.strokeColor.B, m.strokeColor.A,
		)
	}
	renderer.DrawLine(x1, y1, x2, y2)
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

func FPS() uint {
	return fps
}
