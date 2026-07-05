// Command mapgen generates a flat-top hex-grid representation of a map from
// one or two monochromatic PNG masks (land and/or outline) and renders it to a
// PNG and/or exports it as JSON.
//
// Run it as:
//
//	go run ./cmd/mapgen --input-land dem/master-land.png --output-png hex.png
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	"github.com/fogleman/gg"
)

// Layer colors used when merging the logical layers into the final PNG.
var (
	colorLand    = color.NRGBA{R: 211, G: 211, B: 211, A: 255} // light grey
	colorOutline = color.NRGBA{R: 255, G: 0, B: 255, A: 255}   // magenta
	colorGrid    = color.NRGBA{R: 0, G: 0, B: 0, A: 255}       // black
)

// config holds the parsed and validated command-line options.
type config struct {
	inputLand    string
	inputOutline string
	outputPNG    string
	outputJSON   string

	originX    float64
	originY    float64
	originXSet bool
	originYSet bool
	apothem    float64

	renderLand      bool
	renderLandHexes bool
	renderOutline   bool
	renderHexGrid   bool
	borderWidth     int

	emitLand    bool
	emitOcean   bool
	emitClipped bool
}

// hex describes a single hex cell in the generated grid.
type hex struct {
	Q       int     `json:"q"`        // axial Q
	R       int     `json:"r"`        // axial R
	Col     int     `json:"col"`      // offset column, (0,0) is top-left origin hex
	Row     int     `json:"row"`      // offset row
	CenterX float64 `json:"center_x"` // pixel X of hex center
	CenterY float64 `json:"center_y"` // pixel Y of hex center
	IsLand  bool    `json:"is_land"`  // true if the center pixel is inside the land mask
	Clipped bool    `json:"clipped"`  // true if any part of the rendered hex is off-canvas
}

// mapDocument is the top-level JSON structure written to --output-json.
type mapDocument struct {
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	OriginX     float64 `json:"origin_x"`
	OriginY     float64 `json:"origin_y"`
	Apothem     float64 `json:"apothem"`
	CircumRadius float64 `json:"circumradius"`
	Orientation string  `json:"orientation"`
	Offset      string  `json:"offset"`
	HexCount    int     `json:"hex_count"`
	Hexes       []hex   `json:"hexes"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mapgen:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseFlags()
	if err != nil {
		return err
	}

	// Load whichever masks were provided.
	var land, outline *image.NRGBA
	var width, height int

	if cfg.inputLand != "" {
		land, err = loadMask(cfg.inputLand)
		if err != nil {
			return fmt.Errorf("loading --input-land: %w", err)
		}
		width, height = land.Bounds().Dx(), land.Bounds().Dy()
	}
	if cfg.inputOutline != "" {
		outline, err = loadMask(cfg.inputOutline)
		if err != nil {
			return fmt.Errorf("loading --input-outline: %w", err)
		}
		w, h := outline.Bounds().Dx(), outline.Bounds().Dy()
		if land != nil && (w != width || h != height) {
			return fmt.Errorf("input dimensions differ: land is %dx%d, outline is %dx%d", width, height, w, h)
		}
		width, height = w, h
	}

	// Default the origin to the center of the canvas when not set explicitly.
	if !cfg.originXSet {
		cfg.originX = float64(width) / 2
	}
	if !cfg.originYSet {
		cfg.originY = float64(height) / 2
	}

	hexes := generateHexes(cfg, land, width, height)

	if cfg.outputPNG != "" {
		img := renderPNG(cfg, land, outline, hexes, width, height)
		if err := writePNG(cfg.outputPNG, img); err != nil {
			return fmt.Errorf("writing --output-png: %w", err)
		}
	}

	if cfg.outputJSON != "" {
		doc := mapDocument{
			Width:        width,
			Height:       height,
			OriginX:      cfg.originX,
			OriginY:      cfg.originY,
			Apothem:      cfg.apothem,
			CircumRadius: circumRadius(cfg.apothem),
			Orientation:  "flat-top",
			Offset:       "odd-q (odd columns shifted down)",
			HexCount:     len(hexes),
			Hexes:        hexes,
		}
		if err := writeJSON(cfg.outputJSON, doc); err != nil {
			return fmt.Errorf("writing --output-json: %w", err)
		}
	}

	return nil
}

// parseFlags parses the command line, applies the documented defaults and
// forcing rules, and validates the result.
func parseFlags() (config, error) {
	var cfg config

	flag.StringVar(&cfg.inputLand, "input-land", "", "input PNG with a transparent background (land mask)")
	flag.StringVar(&cfg.inputOutline, "input-outline", "", "input PNG with a transparent background (outline mask)")
	flag.StringVar(&cfg.outputPNG, "output-png", "", "output PNG file name")
	flag.StringVar(&cfg.outputJSON, "output-json", "", "output JSON file name")

	flag.Float64Var(&cfg.originX, "origin-x", 0, "X pixel of the hex-grid origin (defaults to input width/2)")
	flag.Float64Var(&cfg.originY, "origin-y", 0, "Y pixel of the hex-grid origin (defaults to input height/2)")
	flag.Float64Var(&cfg.apothem, "apothem", 21.7, "pixel size of the hex apothem (inradius)")

	flag.BoolVar(&cfg.renderLand, "render-land", false, "render solid land (from the mask) in light grey")
	flag.BoolVar(&cfg.renderLandHexes, "render-land-hexes", false, "fill each hex whose center is in the land mask, in light grey")
	flag.BoolVar(&cfg.renderOutline, "render-outline", true, "render the outline in magenta")
	flag.BoolVar(&cfg.renderHexGrid, "render-hex-grid", true, "render the hex grid")
	flag.IntVar(&cfg.borderWidth, "hex-border-width", 3, "hex grid border width in pixels")

	flag.BoolVar(&cfg.emitLand, "emit-land-hexes", true, "include land hexes in the grid and JSON")
	flag.BoolVar(&cfg.emitOcean, "emit-ocean-hexes", true, "include ocean (water) hexes in the grid and JSON")
	flag.BoolVar(&cfg.emitClipped, "emit-clipped-hexes", true, "include clipped (partially off-canvas) hexes in the grid and JSON")

	flag.Parse()

	// Record whether the origin flags were explicitly provided.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "origin-x":
			cfg.originXSet = true
		case "origin-y":
			cfg.originYSet = true
		}
	})

	if cfg.inputLand == "" && cfg.inputOutline == "" {
		return cfg, fmt.Errorf("at least one of --input-land or --input-outline must be specified")
	}
	if cfg.apothem <= 0 {
		return cfg, fmt.Errorf("--apothem must be positive, got %g", cfg.apothem)
	}
	if cfg.borderWidth < 1 {
		return cfg, fmt.Errorf("--hex-border-width must be at least 1, got %d", cfg.borderWidth)
	}

	// Forcing rules: a layer cannot be rendered without its input.
	if cfg.inputLand == "" {
		cfg.renderLand = false
		cfg.renderLandHexes = false
	}
	if cfg.inputOutline == "" {
		cfg.renderOutline = false
	}

	return cfg, nil
}

// circumRadius returns the hexagon circumradius (center to vertex) for a given
// apothem (center to edge midpoint): apothem = R * sqrt(3)/2.
func circumRadius(apothem float64) float64 {
	return 2 * apothem / math.Sqrt(3)
}

// hexCenter returns the pixel center of the flat-top axial hex (q, r).
func hexCenter(cfg config, q, r int) (float64, float64) {
	R := circumRadius(cfg.apothem)
	colSpacing := 1.5 * R              // horizontal distance between columns
	rowSpacing := math.Sqrt(3) * R     // vertical distance between rows (= 2*apothem)
	cx := cfg.originX + colSpacing*float64(q)
	cy := cfg.originY + rowSpacing*(float64(q)/2+float64(r))
	return cx, cy
}

// hexVertices returns the six pixel vertices of a flat-top hexagon centered at
// (cx, cy) with circumradius R, starting at 0 degrees and stepping by 60.
func hexVertices(cx, cy, R float64) [6][2]float64 {
	var v [6][2]float64
	for i := 0; i < 6; i++ {
		a := math.Pi / 3 * float64(i) // 0, 60, 120, ... degrees
		v[i][0] = cx + R*math.Cos(a)
		v[i][1] = cy + R*math.Sin(a)
	}
	return v
}

// generateHexes builds every hex whose bounding box overlaps the canvas, then
// filters the set according to the --emit-* flags.
func generateHexes(cfg config, land *image.NRGBA, width, height int) []hex {
	R := circumRadius(cfg.apothem)
	colSpacing := 1.5 * R
	rowSpacing := math.Sqrt(3) * R

	// Column range that can reach the canvas, with a one-hex margin.
	qMin := int(math.Floor((0-cfg.originX)/colSpacing)) - 1
	qMax := int(math.Ceil((float64(width)-cfg.originX)/colSpacing)) + 1

	var hexes []hex
	for q := qMin; q <= qMax; q++ {
		// Row range for this column that can reach the canvas.
		base := -float64(q) / 2
		rMin := int(math.Floor((0-cfg.originY)/rowSpacing+base)) - 1
		rMax := int(math.Ceil((float64(height)-cfg.originY)/rowSpacing+base)) + 1

		for r := rMin; r <= rMax; r++ {
			cx, cy := hexCenter(cfg, q, r)
			verts := hexVertices(cx, cy, R)

			// Bounding box of the hex.
			minX, minY := verts[0][0], verts[0][1]
			maxX, maxY := verts[0][0], verts[0][1]
			for _, p := range verts {
				minX = math.Min(minX, p[0])
				minY = math.Min(minY, p[1])
				maxX = math.Max(maxX, p[0])
				maxY = math.Max(maxY, p[1])
			}
			// Skip hexes whose bounding box does not overlap the canvas.
			if maxX < 0 || minX > float64(width) || maxY < 0 || minY > float64(height) {
				continue
			}

			// clipped: any vertex lies outside the canvas rectangle.
			clipped := false
			for _, p := range verts {
				if p[0] < 0 || p[0] > float64(width) || p[1] < 0 || p[1] > float64(height) {
					clipped = true
					break
				}
			}

			isLand := maskContains(land, cx, cy, width, height)

			// Apply emission filters.
			if isLand && !cfg.emitLand {
				continue
			}
			if !isLand && !cfg.emitOcean {
				continue
			}
			if clipped && !cfg.emitClipped {
				continue
			}

			col, row := axialToOffset(q, r)
			hexes = append(hexes, hex{
				Q:       q,
				R:       r,
				Col:     col,
				Row:     row,
				CenterX: cx,
				CenterY: cy,
				IsLand:  isLand,
				Clipped: clipped,
			})
		}
	}
	return hexes
}

// axialToOffset converts flat-top axial coordinates to odd-q offset coordinates
// (odd columns shifted down).
func axialToOffset(q, r int) (col, row int) {
	col = q
	row = r + (q-(q&1))/2
	return col, row
}

// maskContains reports whether the pixel at (x, y) is inside the mask, i.e. the
// mask exists, the point is on-canvas, and the alpha channel is at least 128.
func maskContains(mask *image.NRGBA, x, y float64, width, height int) bool {
	if mask == nil {
		return false
	}
	ix, iy := int(math.Round(x)), int(math.Round(y))
	if ix < 0 || ix >= width || iy < 0 || iy >= height {
		return false
	}
	return mask.NRGBAAt(ix, iy).A >= 128
}

// renderPNG merges the logical layers into a single RGBA image:
// transparent background, land, outline, then the hex grid.
func renderPNG(cfg config, land, outline *image.NRGBA, hexes []hex, width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	// Background is already transparent (zero value).

	if cfg.renderLand && land != nil {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if land.NRGBAAt(x, y).A >= 128 {
					img.SetNRGBA(x, y, colorLand)
				}
			}
		}
	}

	if cfg.renderLandHexes {
		dc := gg.NewContextForImage(img)
		dc.SetColor(colorLand)
		R := circumRadius(cfg.apothem)
		for _, h := range hexes {
			if !h.IsLand {
				continue
			}
			verts := hexVertices(h.CenterX, h.CenterY, R)
			dc.MoveTo(verts[0][0], verts[0][1])
			for i := 1; i < 6; i++ {
				dc.LineTo(verts[i][0], verts[i][1])
			}
			dc.ClosePath()
		}
		dc.Fill()

		out := dc.Image()
		draw.Draw(img, img.Bounds(), out, out.Bounds().Min, draw.Src)
	}

	if cfg.renderOutline && outline != nil {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if outline.NRGBAAt(x, y).A >= 128 {
					img.SetNRGBA(x, y, colorOutline)
				}
			}
		}
	}

	if cfg.renderHexGrid {
		dc := gg.NewContextForImage(img)
		dc.SetColor(colorGrid)
		dc.SetLineWidth(float64(cfg.borderWidth))
		R := circumRadius(cfg.apothem)
		for _, h := range hexes {
			verts := hexVertices(h.CenterX, h.CenterY, R)
			dc.MoveTo(verts[0][0], verts[0][1])
			for i := 1; i < 6; i++ {
				dc.LineTo(verts[i][0], verts[i][1])
			}
			dc.ClosePath()
		}
		dc.Stroke()

		// gg renders onto its own buffer; copy the result back into img.
		out := dc.Image()
		draw.Draw(img, img.Bounds(), out, out.Bounds().Min, draw.Src)
	}

	return img
}

// loadMask decodes a PNG file and converts it to *image.NRGBA for uniform,
// fast alpha access.
func loadMask(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}

	if nrgba, ok := src.(*image.NRGBA); ok {
		return nrgba, nil
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst, nil
}

// writePNG encodes img to path as a PNG file.
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// writeJSON writes doc to path as indented JSON.
func writeJSON(path string, doc mapDocument) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
