# hexoverlay

A small command-line utility that overlays a flat-top hex grid onto a PNG image and optionally writes the generated hex coordinates/centers to JSON or CSV.

## Features

- Loads an input PNG with transparency.
- Creates an output PNG with the same dimensions.
- Supports a **flat-top** hex grid using Red Blob's **odd-q vertical layout**.
- The pixel origin `(--origin-x, --origin-y)` maps to axial coordinates `(q=0, r=0)` and odd-q offset coordinates `(col=0, row=0)`.
- Can render:
  - land only
  - hex grid only
  - both together
- Can write generated hex records to JSON or CSV.
- Each generated hex record includes:
  - axial `q`, `r`
  - odd-q offset `col`, `row`
  - center pixel coordinates
  - `is_land`, based on the alpha channel at the hex center
  - `in_canvas`, whether the center point is inside the image canvas

## Flags

- `--input foo.png` input PNG file
- `--output hex.png` output PNG file; optional if `--hex-output` is provided
- `--origin-x 2095` x-coordinate of hex origin in pixels
- `--origin-y 615` y-coordinate of hex origin in pixels
- `--apothem 27.1` apothem of each hex in pixels
- `--no-render-hex-grid` if true, do not render the grid
- `--no-render-land` if true, do not render the land silhouette
- `--hex-output hexes.json` optional JSON or CSV coordinate output
- `--hex-output-format auto|json|csv` coordinate output format; default is `auto`
- `--land-alpha-threshold 1` alpha threshold, 0-255, used to mark a hex center as land

## Build

```bash
go build -o hexoverlay .
```

## Examples

Render land and grid:

```bash
./hexoverlay \
  --input panama-mask.png \
  --output panama-grid.png \
  --origin-x 2095 \
  --origin-y 615 \
  --apothem 27.1
```

Render land and grid, and write JSON coordinates:

```bash
./hexoverlay \
  --input panama-mask.png \
  --output panama-grid.png \
  --hex-output panama-hexes.json \
  --origin-x 2095 \
  --origin-y 615 \
  --apothem 27.1
```

Write CSV coordinates only, with no PNG output:

```bash
./hexoverlay \
  --input panama-mask.png \
  --hex-output panama-hexes.csv \
  --hex-output-format csv \
  --origin-x 2095 \
  --origin-y 615 \
  --apothem 27.1
```

Grid only:

```bash
./hexoverlay \
  --input panama-mask.png \
  --output grid-only.png \
  --origin-x 2095 \
  --origin-y 615 \
  --apothem 27.1 \
  --no-render-land
```

Land only:

```bash
./hexoverlay \
  --input panama-mask.png \
  --output land-only.png \
  --origin-x 2095 \
  --origin-y 615 \
  --apothem 27.1 \
  --no-render-hex-grid
```

## Geometry

Given apothem `a` in pixels:

- side length / circumradius `s = 2a / sqrt(3)`
- point-to-point width `2s`
- flat-to-flat height `2a`
- horizontal center spacing `1.5s`
- vertical center spacing `2a`
- odd-numbered columns are shifted downward by `a`

## Coordinate output

The coordinate output includes every generated hex whose polygon bounding box intersects the canvas.

### JSON shape

```json
{
  "canvas": {
    "width": 2172,
    "height": 724
  },
  "geometry": {
    "layout": "odd-q",
    "orientation": "flat-top",
    "offset": "odd columns shifted down; first column is 0",
    "origin_x": 2095,
    "origin_y": 615,
    "apothem": 27.1,
    "radius": 31.292,
    "width_point_to_point": 62.584,
    "height_flat_to_flat": 54.2,
    "horizontal_center_spacing": 46.938,
    "vertical_center_spacing": 54.2
  },
  "hexes": [
    {
      "q": 0,
      "r": 0,
      "col": 0,
      "row": 0,
      "center_x": 2095,
      "center_y": 615,
      "is_land": false,
      "in_canvas": true
    }
  ]
}
```

### CSV columns

```text
q,r,col,row,center_x,center_y,is_land,in_canvas
```

