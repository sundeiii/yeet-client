//go:build windows

package screenshots

import (
	"fmt"
	"image"
	"unsafe"
)

func captureScreenRect(x, y, width, height int) (*image.NRGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf(
			"invalid screen capture size: %dx%d",
			width,
			height,
		)
	}

	screenDC, err := getDC(0)
	if err != nil {
		return nil, err
	}
	defer releaseDC(0, screenDC)

	memDC, err := createCompatibleDC(screenDC)
	if err != nil {
		return nil, err
	}
	defer deleteDC(memDC)

	bitmap, err := createCompatibleBitmap(screenDC, width, height)
	if err != nil {
		return nil, err
	}
	defer deleteObject(bitmap)

	oldObj, err := selectObject(memDC, bitmap)
	if err != nil {
		return nil, err
	}
	defer selectObject(memDC, oldObj)

	if err := bitBlt(
		memDC,
		0,
		0,
		width,
		height,
		screenDC,
		x,
		y,
		srccopy|captureBlt,
	); err != nil {
		return nil, err
	}

	return bitmapToNRGBA(screenDC, bitmap, width, height)
}

func captureWindow(hwnd uintptr) (*image.NRGBA, error) {
	rect, err := getWindowRect(hwnd)
	if err != nil {
		return nil, err
	}

	width := int(rect.Right - rect.Left)
	height := int(rect.Bottom - rect.Top)

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf(
			"invalid window capture size: %dx%d",
			width,
			height,
		)
	}

	windowDC, err := getWindowDC(hwnd)
	if err != nil {
		return nil, err
	}
	defer releaseDC(hwnd, windowDC)

	memDC, err := createCompatibleDC(windowDC)
	if err != nil {
		return nil, err
	}
	defer deleteDC(memDC)

	bitmap, err := createCompatibleBitmap(windowDC, width, height)
	if err != nil {
		return nil, err
	}
	defer deleteObject(bitmap)

	oldObj, err := selectObject(memDC, bitmap)
	if err != nil {
		return nil, err
	}
	defer selectObject(memDC, oldObj)

	if !printWindow(hwnd, memDC, pwRenderFullContent) {
		if err := bitBlt(
			memDC,
			0,
			0,
			width,
			height,
			windowDC,
			0,
			0,
			srccopy|captureBlt,
		); err != nil {
			return nil, err
		}
	}

	return bitmapToNRGBA(windowDC, bitmap, width, height)
}

func bitmapToNRGBA(hdc uintptr, bitmap uintptr, width, height int) (*image.NRGBA, error) {
	var bmi bitmapInfo
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(width)
	bmi.BmiHeader.BiHeight = -int32(height) // Negative means top-down DIB
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = biRGB

	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	r1, _, e1 := procGetDIBits.Call(
		hdc,
		bitmap,
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&img.Pix[0])),
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
	)
	if r1 == 0 {
		return nil, syscallErr("GetDIBits", e1)
	}

	// Windows DIBs are returned in BGRA format.
	// We need to swap the Blue and Red channels to make it RGBA,
	// and ensure Alpha is fully opaque.
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+2] = img.Pix[i+2], img.Pix[i]
		img.Pix[i+3] = 0xff
	}

	return img, nil
}
