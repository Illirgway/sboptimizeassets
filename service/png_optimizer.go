//
//  Copyright (C) 2024 Illirgway
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <https://www.gnu.org/licenses/>.

package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"sort"
)

// SEE https://repository.root-me.org/St%C3%A9ganographie/EN%20-%20PNG%20(Portable%20Network%20Graphics)%20Specification%20version%201.2.pdf
// SEE https://en.wikipedia.org/wiki/PNG

/*
   Color    Allowed    Interpretation
   Type    Bit Depths
     0     1,2,4,8,16  Each pixel is a grayscale sample.
     2     8,16        Each pixel is an R,G,B triple.
     3     1,2,4,8     Each pixel is a palette index;
                       a PLTE chunk must appear.
     4     8,16        Each pixel is a grayscale sample,
                       followed by an alpha sample.
     6     8,16        Each pixel is an R,G,B triple,
                       followed by an alpha sample.
*/

type PNGOptimizer struct {
	encoder png.Encoder
}

type pngImage struct {
	size int64
	img  image.Image
}

const (
	extPNG = "png"
)

var (
	pngOptimizer = PNGOptimizer{
		encoder: png.Encoder{
			CompressionLevel: png.BestCompression,
			// TODO BufferPool:       nil,
		},
	}
)

func init() {
	registryAssetOptimizer(extPNG, &pngOptimizer)
}

// SEE gg.LoadPNG https://github.com/fogleman/gg/blob/master/util.go
func (o *PNGOptimizer) loadPNG(path string) (_ *pngImage, err error) {

	file, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	fi, err := file.Stat()

	if err != nil {
		return nil, err
	}

	img, err := png.Decode(file)

	if err != nil {
		return nil, err
	}

	return &pngImage{
		size: fi.Size(),
		img:  img,
	}, nil
}

func savePNG(path string, b *bytes.Buffer) (err error) {

	fp, err := os.Create(path)

	if err != nil {
		return err
	}

	defer fp.Close()

	_, err = b.WriteTo(fp)

	return err
}

// NOTE сперва сохраняем временный файл, потом его атомарно mv
func (o *PNGOptimizer) savePNG(path string, b *bytes.Buffer) (err error) {

	dstPath := path + ".pngtmp"

	if err = savePNG(dstPath, b); err != nil {
		return err
	}

	// mv
	if err = os.Rename(dstPath, path); err != nil {
		return err
	}

	return nil
}

// SEE https://github.com/aprimadi/imagecomp

// TODO отчет о количестве сэкономленных байт
func (o *PNGOptimizer) Optimize(path string) (_ uint, err error) {

	// NOTE png.Decode весьма черезжопно работает с особыми случаями типа "RGA / Gray + tRNS transparent color",
	//      считывая их все как NRGBA / NRGBA64
	img, err := o.loadPNG(path)

	if err != nil {
		return 0, fmt.Errorf("PNGOptimizer optimize error: %w", err)
	}

	/* список всех вариантов из png.Decode (go 1.20)
	gray     *image.Gray // cbG1, cbG2, cbG4, cbG8
	rgba     *image.RGBA // cbTC8
	paletted *image.Paletted // cbP1, cbP2, cbP4, cbP8
	nrgba    *image.NRGBA // (cbG1, cbG2, cbG4, cbG8) + useTransparent; cbGA8; cbTC8 + useTransparent; cbTCA8
	gray16   *image.Gray16 // cbG16
	rgba64   *image.RGBA64 // cbTC16
	nrgba64  *image.NRGBA64 // cbTCA16; cbTC16 + useTransparent; cbGA16 + useTransparent; cbG16 + useTransparent
	*/

	var (
		opt *bytes.Buffer
		as  string
	)

	// SEE https://blog.sensecodons.com/2022/10/speed-up-png-encoding-in-go-with-nrgba.html
	// SEE https://repository.root-me.org/St%C3%A9ganographie/EN%20-%20PNG%20(Portable%20Network%20Graphics)%20Specification%20version%201.2.pdf
	//     $ 2.4: "PNG does not use premultiplied alpha."
	//     $ 12.8 Non-premultiplied alpha
	switch v := img.img.(type) {
	case *image.RGBA:
		opt, as, err = o.optimizeRGBA(v)
	case *image.NRGBA:
		opt, as, err = o.optimizeNRGBA(v)
	case *image.Paletted:
		opt, as, err = o.optimizePaletted(v)
	case *image.Gray:
		opt, as, err = o.optimizeGray(v)
	// TODO сделать оптимизации для gray16, rgba64, nrgba64
	default:
		opt = bytes.NewBuffer(nil)

		if err = o.encoder.Encode(opt, v); err != nil {
			return 0, fmt.Errorf("error encode src: %w", err)
		}
	}

	// check error
	if err != nil {
		return 0, err
	}

	sz := int64(opt.Len())
	delta := img.size - sz

	if delta <= 0 { // img.size <= int64(opt.Len())
		fmt.Println(" NOOP")
		return 0, nil
	}

	pct := float64(delta) / float64(img.size) * 100

	fmt.Printf(" SAVE AS %s : %d --> %d == %d bytes (%.2f%%)\n", as, img.size, sz, delta, pct)

	if err = o.savePNG(path, opt); err != nil {
		return 0, err
	}

	return uint(delta), nil
}

func (o *PNGOptimizer) optimizeRGBA(src *image.RGBA) (_ *bytes.Buffer, as string, err error) {
	// https://stackoverflow.com/a/58259978
	b := src.Bounds()
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	return o.optimizeNRGBA(img)
}

func (o *PNGOptimizer) optimizeNRGBA(src *image.NRGBA) (_ *bytes.Buffer, as string, err error) {

	variants := make(variantsList, 0, 4) // src + gray + paletted

	// 0й вариант есть всегда - прямо сжатие src
	{
		b := bytes.NewBuffer(nil)

		if err = o.encoder.Encode(b, src); err != nil {
			return nil, "", fmt.Errorf("error encode src: %w", err)
		}

		variants = append(variants, variant{b, "src (nrgba/rgb)"})
	}

	nColors, hasTransparent, hasPartAlpha, isGray := o.countNRGBAColors(src)

	hasAlpha := hasTransparent || hasPartAlpha

	if isGray && !hasAlpha {

		b, gray := bytes.NewBuffer(nil), o.nrgba2gray(src)

		if err = o.encoder.Encode(b, gray); err != nil {
			return nil, "", fmt.Errorf("error encode gray: %w", err)
		}

		variants = append(variants, variant{b, "gray"})
	}

	// TODO? граничный случай с 0 цветов

	// NOTE на текущий момент (go 1.20) голанг png.Encode умеет либо PLTE+tRNS, либо Alpha-channel (gray or rgb),
	//      но НЕ умеет rga + tRNS, gray + tRNS, это упрощает алгоритм, но убивает оптимизацию очень маленьких
	//		насыщенных цветом изображений (число цветов ~= числу пикселей), у которых есть 1 прозрачный альфа цвет
	//      (transparent)
	//      ПРИЧЕМ png.Decode при этом понимает такие особые случаи и умеет с ними работать, см. Optimize()

	// TODO на самом деле должны сравнивать

	// Indexed-color images of up to 256 colors.
	if nColors <= 256 {

		var b *bytes.Buffer

		// SEE https://stackoverflow.com/questions/35850753/how-to-convert-image-rgba-image-image-to-image-paletted
		if b, err = o.asPaletted(src, o.paletteFromNRGBA(src, nColors)); err != nil {
			return nil, "", err
		}

		variants = append(variants, variant{b, "paletted"})
	}

	return variants.best()
}

// без учета серых изображений, есть 3 основных варианта сохранения цветных изображений как RGBA:
// - альфа цвета нет вообще
// - есть ровно 1 альфа цвет - полная прозрачность
// - есть несколько (от 1 и больше) полупрозрачных цветов, что может включать в себя также полную прозрачность
// для каждого из которых имеет смысл собственная особая обработка
func (o *PNGOptimizer) countNRGBAColors(img *image.NRGBA) (n uint, hasTransparent, hasPartAlpha, isGray bool) {

	isGray = true
	hasTransparent = false
	hasPartAlpha = false

	// NOTE key as color.NRGBA concrete struct faster than key of color.Color abstract iface
	// TODO key as uint64
	colors := make(map[color.NRGBA]struct{})

	bounds := img.Bounds()

	// SEE https://github.com/KEINOS/go-pallet/blob/main/pallet/pallet.go
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.NRGBAAt(x, y)

			// TODO для `c.A === 0` надо унифицировать цвет к {0, 0, 0, 0}, потому что визуально
			//      это полная прозрачность для любых значений RGB
			//  но для этого сперва надо проверить, правильно ли работают алгоритмы сохранения палитры в png save,
			//  плюс преобразовывать в таком случае оригинал при сохранении в RGBA, подменяя все transparent цвета,
			//  если они не соответствуют {0, 0, 0, 0}

			colors[c] = struct{}{}

			// проверяем серость
			// if r, g, b, _ := c.RGBA(); r != g || r != b || g != b {
			if c.R != c.G || c.R != c.B || c.G != c.B {
				isGray = false
			}

			// проверяем альфа канал
			if c.A < math.MaxUint8 {
				if c.A == 0 { // полностью прозрачный
					hasTransparent = true
				} else { // полупрозрачный
					hasPartAlpha = true
				}
			}
		}
	}

	return uint(len(colors)), hasTransparent, hasPartAlpha, isGray
}

func (*PNGOptimizer) paletteFromNRGBA(img *image.NRGBA, hint uint) (palette color.Palette) {

	if hint == 0 {
		hint = 256
	}

	bounds := img.Bounds()

	colors := make(map[color.NRGBA]uint, hint)

	// SEE https://github.com/KEINOS/go-pallet/blob/main/pallet/pallet.go
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {

			c := img.NRGBAAt(x, y)

			// TODO SEE TODO countNRGBAColors

			colors[c] = colors[c] + 1
		}
	}

	// TODO вставка сортировкой, тогда не понадобится отдельная сортировка

	rawPalette := make([]color.NRGBA, 0, len(colors))

	for c := range colors {
		rawPalette = append(rawPalette, c)
	}

	s := &nrgbaPaletteSorter{colors, rawPalette}

	sort.Sort(s)

	// т.к. rawPalette по сути ptr to bakary, то после сортировки посредством nrgbaPaletteSorter сама rawPalette тоже будет отсортирована

	// NRGA -> Color
	palette = make(color.Palette, len(rawPalette))

	for i := range rawPalette {
		palette[i] = rawPalette[i]
	}

	return palette
}

func (o *PNGOptimizer) nrgba2gray(img *image.NRGBA) (gray *image.Gray) {

	bounds := img.Bounds()

	gray = image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	// EMULATE draw.Draw(gray, gray.Bounds(), img, bounds.Min, draw.Src)
	// избегаем преобразования через gray color model, потому что img точно gray, закодированный как NRGBA
	// SEE https://stackoverflow.com/questions/41350255/convert-an-image-to-grayscale-in-go
	// SEE https://rentafounder.com/convert-image-to-grayscale-golang/
	// SEE https://stackoverflow.com/questions/42516203/converting-rgba-image-to-grayscale-golang
	for sy, dy := bounds.Min.Y, 0; sy < bounds.Max.Y; sy++ {
		for sx, dx := bounds.Min.X, 0; sx < bounds.Max.X; sx++ {
			gray.SetGray(dx, dy, color.Gray{Y: img.NRGBAAt(sx, sy).R})
			dx++
		}
		dy++
	}

	return gray
}

func (o *PNGOptimizer) optimizePaletted(src *image.Paletted) (_ *bytes.Buffer, as string, err error) {

	variants := make(variantsList, 0, 2)

	{
		b := bytes.NewBuffer(nil)

		if err = o.encoder.Encode(b, src); err != nil {
			return nil, "", fmt.Errorf("error encode src: %w", err)
		}

		variants = append(variants, variant{b, "src (paletted)"})
	}

	if o.isGrayPalette(src.Palette) {

		b, gray := bytes.NewBuffer(nil), o.paletted2gray(src)

		if err = o.encoder.Encode(b, gray); err != nil {
			return nil, "", fmt.Errorf("error encode gray: %w", err)
		}

		variants = append(variants, variant{b, "gray"})
	}

	return variants.best()
}

func (o *PNGOptimizer) isGrayPalette(palette color.Palette) bool {

	for i := range palette {
		// NOTE наличие полупрозрачногое цвета автоматом делает бесполезной попытку представить в виде gray
		if r, g, b, a := palette[i].RGBA(); a < math.MaxUint16 || r != g || r != b || g != b {
			return false
		}
	}

	return true
}

func (o *PNGOptimizer) paletted2gray(img *image.Paletted) (gray *image.Gray) {

	bounds := img.Bounds()

	gray = image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	// EMULATE draw.Draw(gray, gray.Bounds(), img, bounds.Min, draw.Src)
	// избегаем преобразования через gray color model, потому что img точно gray, закодированный как Paletted
	// SEE https://stackoverflow.com/questions/41350255/convert-an-image-to-grayscale-in-go
	// SEE https://rentafounder.com/convert-image-to-grayscale-golang/
	// SEE https://stackoverflow.com/questions/42516203/converting-rgba-image-to-grayscale-golang
	for sy, dy := bounds.Min.Y, 0; sy < bounds.Max.Y; sy++ {
		for sx, dx := bounds.Min.X, 0; sx < bounds.Max.X; sx++ {
			r, _, _, _ := img.Palette[img.ColorIndexAt(sx, sy)].RGBA()
			gray.SetGray(dx, dy, color.Gray{Y: uint8(r >> 8)})
			dx++
		}
		dy++
	}

	return gray
}

func (o *PNGOptimizer) optimizeGray(src *image.Gray) (_ *bytes.Buffer, as string, err error) {

	variants := make(variantsList, 0, 2)

	{
		b := bytes.NewBuffer(nil)

		if err = o.encoder.Encode(b, src); err != nil {
			return nil, "", fmt.Errorf("error encode src: %w", err)
		}

		variants = append(variants, variant{b, "src (gray)"})
	}

	if nColors := o.countGrayColors(src); nColors <= 256 {

		var b *bytes.Buffer

		if b, err = o.asPaletted(src, o.paletteFromGray(src, nColors)); err != nil {
			return nil, "", err
		}

		variants = append(variants, variant{b, "paletted"})
	}

	return variants.best()
}

func (o *PNGOptimizer) countGrayColors(img *image.Gray) uint {

	bounds := img.Bounds()

	colors := make(map[uint8]struct{})

	// SEE https://github.com/KEINOS/go-pallet/blob/main/pallet/pallet.go
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.GrayAt(x, y)
			colors[c.Y] = struct{}{}
		}
	}

	return uint(len(colors))
}

func (o *PNGOptimizer) paletteFromGray(img *image.Gray, hint uint) (palette color.Palette) {

	if hint == 0 {
		hint = 256
	}

	bounds := img.Bounds()

	colors := make(map[uint8]uint, hint)

	// SEE https://github.com/KEINOS/go-pallet/blob/main/pallet/pallet.go
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.GrayAt(x, y)
			colors[c.Y] = colors[c.Y] + 1
		}
	}

	//

	rawPalette := make([]uint8, 0, len(colors))

	for c := range colors {
		rawPalette = append(rawPalette, c)
	}

	// упорядочиваем по частоте применения (от более частой к менее частой)
	sort.Slice(rawPalette, func(i, j int) bool {
		return colors[rawPalette[i]]-colors[rawPalette[j]] > 0
	})

	palette = make(color.Palette, len(rawPalette))

	for i, c := range rawPalette {
		palette[i] = color.Gray{Y: c}
	}

	return palette
}

func (o *PNGOptimizer) asPaletted(src image.Image, palette color.Palette) (b *bytes.Buffer, err error) {

	bounds := src.Bounds()

	paletted := image.NewPaletted(image.Rect(0, 0, bounds.Dx(), bounds.Dy()), palette)
	draw.Draw(paletted, paletted.Bounds(), src, bounds.Min, draw.Src)

	b = bytes.NewBuffer(nil)

	if err = o.encoder.Encode(b, paletted); err != nil {
		return nil, fmt.Errorf("error encode paletted: %w", err)
	}

	return b, nil
}

/*
func NewPNGOptimizer() *PNGOptimizer {
	return &PNGOptimizer{encoder: png.Encoder{
		CompressionLevel: png.BestCompression,
		// TODO BufferPool:
	}}
}
*/

// implements sort.Interface
type nrgbaPaletteSorter struct {
	colors  map[color.NRGBA]uint // ptr
	palette []color.NRGBA        // ptr
}

func (ps *nrgbaPaletteSorter) Len() int {
	return len(ps.palette)
}

func (ps *nrgbaPaletteSorter) Swap(i, j int) {
	ps.palette[i], ps.palette[j] = ps.palette[j], ps.palette[i]
}

func (ps *nrgbaPaletteSorter) Less(i, j int) bool {

	// NOTE
	// - прозрачный (transparent) всегда самый первый
	// - альфа цвета имеют преимущество над не альфами
	// - при сравнении 2 альфа-цветов (кроме прозрачного) либо 2 не-альфа цветов - согласно частоте появления,
	//   самые частые идут в начало (т.е. более частый less (<) менее частого)
	// такое упорядочивание необходимо, чтобы сделать в PNG таблицу tRNS как можно меньшего размера
	// SEE https://repository.root-me.org/St%C3%A9ganographie/EN%20-%20PNG%20(Portable%20Network%20Graphics)%20Specification%20version%201.2.pdf
	//     $ 4.2.1, 4.2.1.1:
	//     "tRNS can contain fewer values than there are palette entries. In this case, the alpha value for all
	//     remaining palette entries is assumed to be 255. In the common case in which only palette index 0 need be
	//     made transparent, only a one-byte tRNS chunk is needed."
	ci, cj := ps.palette[i], ps.palette[j]

	// fast-path одинаковая альфа - сравнивается по частоте
	if ci.A == cj.A {
		return ps.colors[ci]-ps.colors[cj] > 0
	}

	// HERE ci.A != cj.A

	if ci.A == 0 { // прозрачный первым
		return true
	}

	if cj.A == 0 { // никогда не раньше прозрачного
		return false
	}

	// HERE ci.A != 0 && cj.A != 0 && ci.A != cj.A

	// альфа раньше не альфы
	if ci.A < math.MaxUint8 && cj.A == math.MaxUint8 {
		return true
	}

	// 2 альфа либо 2 не-альфы - по их частотам вхождения
	if (ci.A < math.MaxUint8 && cj.A < math.MaxUint8) || (ci.A == math.MaxUint8 && cj.A == math.MaxUint8) {
		return ps.colors[ci]-ps.colors[cj] > 0
	}

	// все остальное консервативно false
	return false
}

//

type variant struct {
	b  *bytes.Buffer
	as string
}

type variantsList []variant

var (
	errNoVariants = errors.New("unexpected error: empty variants")
)

func (v variantsList) best() (b *bytes.Buffer, as string, err error) {

	if len(v) == 0 {
		return nil, "", errNoVariants
	}

	b, as = v[0].b, v[0].as

	for i := 1; i < len(v); i++ {
		if vv := &v[i]; vv.b.Len() < b.Len() {
			b = vv.b
			as = vv.as
		}
	}

	return b, as, nil
}
