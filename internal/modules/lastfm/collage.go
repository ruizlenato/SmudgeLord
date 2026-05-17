package lastfm

import (
	"bytes"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/anthonynsimon/bild/transform"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	lastFMAPI "github.com/ruizlenato/smudgelord/internal/modules/lastfm/api"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	defaultCollageGridSize = 3
	collageTileSize        = 256
	collageJPEGQual        = 88
	textPaddingX           = 10
	artistFontSize         = 14
	titleFontSize          = 12
	playsFontSize          = 14
)

var (
	//go:embed fonts/NimbusSans-Bold.ttf
	nimbusSansBoldTTF []byte

	artistFace font.Face
	titleFace  font.Face
	playsFace  font.Face
)

func init() {
	boldParsed := parseFontWithFallback(nimbusSansBoldTTF, gobold.TTF)
	if boldParsed == nil {
		return
	}

	artistFace, _ = opentype.NewFace(boldParsed, &opentype.FaceOptions{Size: artistFontSize, DPI: 72, Hinting: font.HintingFull})
	titleFace, _ = opentype.NewFace(boldParsed, &opentype.FaceOptions{Size: titleFontSize, DPI: 72, Hinting: font.HintingFull})
	playsFace, _ = opentype.NewFace(boldParsed, &opentype.FaceOptions{Size: playsFontSize, DPI: 72, Hinting: font.HintingFull})
}

func parseFontWithFallback(primary, fallback []byte) *opentype.Font {
	if len(primary) > 0 {
		if parsed, err := opentype.Parse(primary); err == nil {
			return parsed
		}
	}
	if len(fallback) > 0 {
		if parsed, err := opentype.Parse(fallback); err == nil {
			return parsed
		}
	}
	return nil
}

func buildLastFMCollage(collageType, period, username string, gridSize int, withText bool) ([]byte, error) {
	if gridSize <= 0 {
		gridSize = defaultCollageGridSize
	}

	items, err := getTopItemsForCollage(collageType, period, username, gridSize*gridSize)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no top items")
	}

	canvasW := gridSize * collageTileSize
	canvasH := gridSize * collageTileSize
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{20, 20, 24, 255}}, image.Point{}, draw.Src)

	type tileResult struct {
		idx  int
		til  *image.RGBA
		item lastFMAPI.TopCollageItem
	}
	results := make(chan tileResult, gridSize*gridSize)
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	for i := 0; i < gridSize*gridSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tile := image.NewRGBA(image.Rect(0, 0, collageTileSize, collageTileSize))
			draw.Draw(tile, tile.Bounds(), &image.Uniform{C: color.RGBA{38, 38, 44, 255}}, image.Point{}, draw.Src)

			if idx < len(items) {
				item := items[idx]
				if img, imgErr := fetchCollageTile(item, collageType, collageTileSize); imgErr == nil && img != nil {
					draw.Draw(tile, tile.Bounds(), img, img.Bounds().Min, draw.Over)
				}

				results <- tileResult{idx: idx, til: tile, item: item}
				return
			}

			results <- tileResult{idx: idx, til: tile}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		row := res.idx / gridSize
		col := res.idx % gridSize
		x := col * collageTileSize
		y := row * collageTileSize
		if withText && res.item.Title != "" {
			drawTileLabel(res.til, res.item, collageTileSize)
		}
		draw.Draw(canvas, image.Rect(x, y, x+collageTileSize, y+collageTileSize), res.til, image.Point{}, draw.Src)
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: collageJPEGQual}); err != nil {
		return nil, fmt.Errorf("failed to encode collage: %w", err)
	}

	return out.Bytes(), nil
}

func getTopItemsForCollage(collageType, period, username string, limit int) ([]lastFMAPI.TopCollageItem, error) {
	return lastFM.GetTopCollageItems(collageType, username, period, limit)
}

func fetchCollageImage(url string) (image.Image, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("empty image url")
	}

	cacheKey := collageImageCacheKey(url)
	if raw, err := cache.GetCacheBytes(cacheKey); err == nil && len(raw) > 0 {
		img, _, decodeErr := image.Decode(bytes.NewReader(raw))
		if decodeErr == nil {
			return img, nil
		}
	}

	resp, err := utils.Request(url, utils.RequestParams{Method: http.MethodGet})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image request failed with status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty image body")
	}

	_ = cache.SetCacheBytes(cacheKey, raw, 12*time.Hour)

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func fetchCollageTile(item lastFMAPI.TopCollageItem, collageType string, size int) (image.Image, error) {
	url := item.ImageURL
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("empty image url")
	}
	if size <= 0 {
		size = collageTileSize
	}

	tileKey := collageTileCacheKey(item, collageType, size)
	if raw, err := cache.GetCacheBytes(tileKey); err == nil && len(raw) > 0 {
		img, _, decodeErr := image.Decode(bytes.NewReader(raw))
		if decodeErr == nil {
			return img, nil
		}
	}

	img, err := fetchCollageImage(url)
	if err != nil {
		return nil, err
	}

	resized := transform.Resize(img, size, size, transform.CatmullRom)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, resized, &jpeg.Options{Quality: 90}); err == nil {
		_ = cache.SetCacheBytes(tileKey, out.Bytes(), 24*time.Hour)
	}

	return resized, nil
}

func collageImageCacheKey(rawURL string) string {
	h := sha1.Sum([]byte(rawURL))
	return "lastfm:img:" + hex.EncodeToString(h[:])
}

func collageTileCacheKey(item lastFMAPI.TopCollageItem, collageType string, size int) string {
	if collageType == "artist" {
		artist := normalizedKey(item.Title)
		if artist != "" {
			return fmt.Sprintf("lastfm:img:tile:artist:%d:%s", size, artist)
		}
	}

	return collageTileCacheKeyFromURL(item.ImageURL, size)
}

func collageTileCacheKeyFromURL(rawURL string, size int) string {
	h := sha1.Sum([]byte(rawURL))
	return fmt.Sprintf("lastfm:img:tile:%d:%s", size, hex.EncodeToString(h[:]))
}

func normalizedKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), "_")
}

func drawTileLabel(dst *image.RGBA, item lastFMAPI.TopCollageItem, tileSize int) {
	applyBottomGradient(dst, tileSize/2, color.RGBA{0, 0, 0, 0}, color.RGBA{0, 0, 0, 185})

	artist := trimLabel(item.Subtitle, 29)
	title := trimLabel(item.Title, 31)
	plays := fmt.Sprintf("%d", max(item.Playcount, 0))

	if artist != "" {
		drawTextShadow(dst, artist, textPaddingX, tileSize-30, artistFace)
	}
	drawTextShadow(dst, title, textPaddingX, tileSize-14, titleFace)

	if playsFace != nil {
		d := &font.Drawer{Dst: dst, Src: image.NewUniform(color.RGBA{255, 255, 255, 255}), Face: playsFace}
		w := d.MeasureString(plays)
		x := tileSize - textPaddingX - (w.Ceil())
		drawTextShadow(dst, plays, x, tileSize-14, playsFace)
	}
}

func drawTextShadow(dst *image.RGBA, text string, x, y int, face font.Face) {
	if face == nil || text == "" {
		return
	}
	shadow := &font.Drawer{Dst: dst, Src: image.NewUniform(color.RGBA{0, 0, 0, 220}), Face: face, Dot: fixed.P(x+1, y+1)}
	shadow.DrawString(text)
	main := &font.Drawer{Dst: dst, Src: image.NewUniform(color.RGBA{255, 255, 255, 255}), Face: face, Dot: fixed.P(x, y)}
	main.DrawString(text)
}

func applyBottomGradient(dst *image.RGBA, startY int, from, to color.RGBA) {
	b := dst.Bounds()
	h := b.Dy() - startY
	if h <= 0 {
		return
	}
	for y := startY; y < b.Dy(); y++ {
		t := float64(y-startY) / float64(h)
		a := uint8(float64(from.A) + (float64(to.A-from.A) * t))
		line := image.Rect(0, y, b.Dx(), y+1)
		draw.Draw(dst, line, &image.Uniform{C: color.RGBA{0, 0, 0, a}}, image.Point{}, draw.Over)
	}
}

func trimLabel(value string, maxLen int) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	r := []rune(v)
	if len(r) <= maxLen {
		return v
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(r[:maxLen-1]) + "…"
}

func max(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}
