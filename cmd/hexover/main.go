package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	InputFile          string
	OutputFile         string
	OriginX            float64
	OriginY            float64
	Apothem            float64
	NoRenderHexGrid    bool
	NoRenderLand       bool
	HexOutputFile      string
	HexOutputFormat    string
	LandAlphaThreshold uint
}

type Geometry struct {
	Layout                  string  `json:"layout"`
	Orientation             string  `json:"orientation"`
	Offset                  string  `json:"offset"`
	OriginX                 float64 `json:"origin_x"`
	OriginY                 float64 `json:"origin_y"`
	Apothem                 float64 `json:"apothem"`
	Radius                  float64 `json:"radius"`
	WidthPointToPoint       float64 `json:"width_point_to_point"`
	HeightFlatToFlat        float64 `json:"height_flat_to_flat"`
	HorizontalCenterSpacing float64 `json:"horizontal_center_spacing"`
	VerticalCenterSpacing   float64 `json:"vertical_center_spacing"`
}

type Canvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type HexRecord struct {
	Q        int     `json:"q"`
	R        int     `json:"r"`
	Col      int     `json:"col"`
	Row      int     `json:"row"`
	CenterX  float64 `json:"center_x"`
	CenterY  float64 `json:"center_y"`
	IsLand   bool    `json:"is_land"`
	InCanvas bool    `json:"in_canvas"`
}

type HexOutput struct {
	Canvas   Canvas      `json:"canvas"`
	Geometry Geometry    `json:"geometry"`
	Hexes    []HexRecord `json:"hexes"`
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() Config {
	var cfg Config

	flag.StringVar(&cfg.InputFile, "input", "", "input PNG file with transparent background")
	flag.StringVar(&cfg.OutputFile, "output", "", "output PNG file to create")
	flag.Float64Var(&cfg.OriginX, "origin-x", 0, "pixel x-coordinate of the hex origin, mapping to axial (0,0)")
	flag.Float64Var(&cfg.OriginY, "origin-y", 0, "pixel y-coordinate of the hex origin, mapping to axial (0,0)")
	flag.Float64Var(&cfg.Apothem, "apothem", 21.7, "hex apothem in pixels")
	flag.BoolVar(&cfg.NoRenderHexGrid, "no-render-hex-grid", false, "when true, do not render the hex grid")
	flag.BoolVar(&cfg.NoRenderLand, "no-render-land", false, "when true, do not render the land silhouette")
	flag.StringVar(&cfg.HexOutputFile, "hex-output", "", "optional JSON or CSV file to write generated hex coordinates/centers")
	flag.StringVar(&cfg.HexOutputFormat, "hex-output-format", "auto", "coordinate output format: auto, json, or csv")
	flag.UintVar(&cfg.LandAlphaThreshold, "land-alpha-threshold", 1, "alpha threshold, 0-255, used to mark a hex center as land")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s --input foo.png --output hex.png [options]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintln(flag.CommandLine.Output(), "Creates a PNG with the same size as the input image, using a flat-top hex grid")
		fmt.Fprintln(flag.CommandLine.Output(), "with odd columns shifted down (Red Blob odd-q vertical layout).")
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "The coordinate output contains every generated hex whose polygon intersects the canvas.")
		fmt.Fprintln(flag.CommandLine.Output(), "It includes axial q/r, odd-q offset col/row, center pixel coordinates, and an is_land flag.")
		fmt.Fprintln(flag.CommandLine.Output())
		flag.PrintDefaults()
	}

	flag.Parse()
	return cfg
}

func run(cfg Config) error {
	if err := validate(cfg); err != nil {
		return err
	}

	src, err := loadPNG(cfg.InputFile)
	if err != nil {
		return fmt.Errorf("load input: %w", err)
	}

	bounds := src.Bounds()
	geometry := hexGeometry(cfg.OriginX, cfg.OriginY, cfg.Apothem)
	hexes := collectHexes(src, geometry, cfg.LandAlphaThreshold)

	if cfg.OutputFile != "" {
		out := image.NewNRGBA(bounds)

		if !cfg.NoRenderLand {
			draw.Draw(out, bounds, src, bounds.Min, draw.Over)
		}

		if !cfg.NoRenderHexGrid {
			gridColor := color.NRGBA{R: 0, G: 0, B: 0, A: 128}
			for _, hex := range hexes {
				drawFlatTopHex(out, hex.CenterX, hex.CenterY, geometry.Radius, geometry.Apothem, gridColor)
			}
		}

		if err := savePNG(cfg.OutputFile, out); err != nil {
			return fmt.Errorf("save output: %w", err)
		}
	}

	if cfg.HexOutputFile != "" {
		hexOut := HexOutput{
			Canvas: Canvas{
				Width:  bounds.Dx(),
				Height: bounds.Dy(),
			},
			Geometry: geometry,
			Hexes:    hexes,
		}
		if err := saveHexOutput(cfg.HexOutputFile, cfg.HexOutputFormat, hexOut); err != nil {
			return fmt.Errorf("save hex output: %w", err)
		}
	}

	return nil
}

func validate(cfg Config) error {
	if cfg.InputFile == "" {
		return errors.New("--input is required")
	}
	if cfg.OutputFile == "" && cfg.HexOutputFile == "" {
		return errors.New("at least one of --output or --hex-output is required")
	}
	if cfg.Apothem <= 0 {
		return errors.New("--apothem must be greater than zero")
	}
	if cfg.LandAlphaThreshold > 255 {
		return errors.New("--land-alpha-threshold must be between 0 and 255")
	}
	format := strings.ToLower(cfg.HexOutputFormat)
	if format != "auto" && format != "json" && format != "csv" {
		return errors.New("--hex-output-format must be auto, json, or csv")
	}
	return nil
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func savePNG(path string, img image.Image) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

func saveHexOutput(path, format string, hexOut HexOutput) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}

	format = resolveHexOutputFormat(path, format)
	switch format {
	case "json":
		return saveHexJSON(path, hexOut)
	case "csv":
		return saveHexCSV(path, hexOut)
	default:
		return fmt.Errorf("unsupported hex output format %q", format)
	}
}

func resolveHexOutputFormat(path, format string) string {
	format = strings.ToLower(format)
	if format != "auto" {
		return format
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return "csv"
	default:
		return "json"
	}
}

func saveHexJSON(path string, hexOut HexOutput) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(hexOut)
}

func saveHexCSV(path string, hexOut HexOutput) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{
		"q",
		"r",
		"col",
		"row",
		"center_x",
		"center_y",
		"is_land",
		"in_canvas",
	}); err != nil {
		return err
	}

	for _, h := range hexOut.Hexes {
		rec := []string{
			strconv.Itoa(h.Q),
			strconv.Itoa(h.R),
			strconv.Itoa(h.Col),
			strconv.Itoa(h.Row),
			strconv.FormatFloat(h.CenterX, 'f', 3, 64),
			strconv.FormatFloat(h.CenterY, 'f', 3, 64),
			strconv.FormatBool(h.IsLand),
			strconv.FormatBool(h.InCanvas),
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}

	return w.Error()
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func hexGeometry(originX, originY, apothem float64) Geometry {
	radius := 2 * apothem / math.Sqrt(3)
	return Geometry{
		Layout:                  "odd-q",
		Orientation:             "flat-top",
		Offset:                  "odd columns shifted down; first column is 0",
		OriginX:                 originX,
		OriginY:                 originY,
		Apothem:                 apothem,
		Radius:                  radius,
		WidthPointToPoint:       2 * radius,
		HeightFlatToFlat:        2 * apothem,
		HorizontalCenterSpacing: 1.5 * radius,
		VerticalCenterSpacing:   2 * apothem,
	}
}

// collectHexes returns every hex whose polygon bounding box intersects the canvas.
// Coordinates:
//   - Col/Row are Red Blob odd-q offset coordinates.
//   - Q/R are axial coordinates derived from the odd-q offset coordinates.
func collectHexes(src image.Image, geom Geometry, alphaThreshold uint) []HexRecord {
	b := src.Bounds()
	w := float64(b.Dx())
	h := float64(b.Dy())

	minCol := int(math.Floor((0-geom.OriginX)/geom.HorizontalCenterSpacing)) - 2
	maxCol := int(math.Ceil((w-geom.OriginX)/geom.HorizontalCenterSpacing)) + 2

	var hexes []HexRecord

	for col := minCol; col <= maxCol; col++ {
		cx := geom.OriginX + geom.HorizontalCenterSpacing*float64(col)
		colOffset := 0.0
		if isOdd(col) {
			colOffset = geom.Apothem
		}

		minRow := int(math.Floor((0-(geom.OriginY+colOffset))/geom.VerticalCenterSpacing)) - 2
		maxRow := int(math.Ceil((h-(geom.OriginY+colOffset))/geom.VerticalCenterSpacing)) + 2

		for row := minRow; row <= maxRow; row++ {
			cy := geom.OriginY + geom.VerticalCenterSpacing*float64(row) + colOffset

			// Keep any hex whose polygon intersects the image canvas.
			if cx+geom.Radius < 0 || cx-geom.Radius > w || cy-geom.Apothem > h || cy+geom.Apothem < 0 {
				continue
			}

			q, r := offsetOddQToAxial(col, row)
			inCanvas := pointInBounds(b, cx, cy)
			isLand := false
			if inCanvas {
				isLand = centerIsLand(src, cx, cy, alphaThreshold)
			}

			hexes = append(hexes, HexRecord{
				Q:        q,
				R:        r,
				Col:      col,
				Row:      row,
				CenterX:  cx,
				CenterY:  cy,
				IsLand:   isLand,
				InCanvas: inCanvas,
			})
		}
	}

	return hexes
}

// offsetOddQToAxial converts Red Blob odd-q offset coordinates to axial coordinates.
// odd-q means odd columns are shifted downward.
func offsetOddQToAxial(col, row int) (q, r int) {
	parity := col & 1
	q = col
	r = row - (col-parity)/2
	return q, r
}

func pointInBounds(b image.Rectangle, x, y float64) bool {
	ix := int(math.Round(x))
	iy := int(math.Round(y))
	return image.Pt(ix, iy).In(b)
}

func centerIsLand(img image.Image, x, y float64, alphaThreshold uint) bool {
	ix := int(math.Round(x))
	iy := int(math.Round(y))
	_, _, _, a := img.At(ix, iy).RGBA()
	alpha8 := a >> 8
	return alpha8 >= uint32(alphaThreshold)
}

func isOdd(n int) bool {
	return n&1 != 0
}

func drawFlatTopHex(dst *image.NRGBA, cx, cy, radius, apothem float64, c color.NRGBA) {
	verts := [6][2]float64{
		{cx + radius, cy},
		{cx + radius/2, cy + apothem},
		{cx - radius/2, cy + apothem},
		{cx - radius, cy},
		{cx - radius/2, cy - apothem},
		{cx + radius/2, cy - apothem},
	}

	for i := 0; i < 6; i++ {
		a := verts[i]
		b := verts[(i+1)%6]
		drawLine(dst, a[0], a[1], b[0], b[1], c)
	}
}

func drawLine(dst *image.NRGBA, x0, y0, x1, y1 float64, c color.NRGBA) {
	ix0 := int(math.Round(x0))
	iy0 := int(math.Round(y0))
	ix1 := int(math.Round(x1))
	iy1 := int(math.Round(y1))

	dx := absInt(ix1 - ix0)
	sx := -1
	if ix0 < ix1 {
		sx = 1
	}
	dy := -absInt(iy1 - iy0)
	sy := -1
	if iy0 < iy1 {
		sy = 1
	}
	err := dx + dy

	for {
		blendPixel(dst, ix0, iy0, c)
		if ix0 == ix1 && iy0 == iy1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			ix0 += sx
		}
		if e2 <= dx {
			err += dx
			iy0 += sy
		}
	}
}

func blendPixel(dst *image.NRGBA, x, y int, src color.NRGBA) {
	if !image.Pt(x, y).In(dst.Bounds()) {
		return
	}

	i := dst.PixOffset(x, y)
	dr := uint32(dst.Pix[i+0])
	dg := uint32(dst.Pix[i+1])
	db := uint32(dst.Pix[i+2])
	da := uint32(dst.Pix[i+3])

	sa := uint32(src.A)
	invSA := 255 - sa

	outA := sa + (da*invSA+127)/255
	if outA == 0 {
		dst.Pix[i+0] = 0
		dst.Pix[i+1] = 0
		dst.Pix[i+2] = 0
		dst.Pix[i+3] = 0
		return
	}

	sr := uint32(src.R)
	sg := uint32(src.G)
	sb := uint32(src.B)

	// Alpha blend in straight-alpha space.
	outR := (sr*sa + (dr*da*invSA+127)/255)
	outG := (sg*sa + (dg*da*invSA+127)/255)
	outB := (sb*sa + (db*da*invSA+127)/255)

	dst.Pix[i+0] = uint8((outR + outA/2) / outA)
	dst.Pix[i+1] = uint8((outG + outA/2) / outA)
	dst.Pix[i+2] = uint8((outB + outA/2) / outA)
	dst.Pix[i+3] = uint8(outA)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
