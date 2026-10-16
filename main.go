package main

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// PDF Color Checker 5.0. Native, self-contained Windows GUI.
// Requires no embedded browser, .NET, Python or external DLLs.
var (
	user32            = syscall.NewLazyDLL("user32.dll")
	gdi32             = syscall.NewLazyDLL("gdi32.dll")
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	shell32           = syscall.NewLazyDLL("shell32.dll")
	ole32             = syscall.NewLazyDLL("ole32.dll")
	pRegisterClass    = user32.NewProc("RegisterClassW")
	pCreateWindow     = user32.NewProc("CreateWindowExW")
	pDefWindow        = user32.NewProc("DefWindowProcW")
	pShowWindow       = user32.NewProc("ShowWindow")
	pUpdateWindow     = user32.NewProc("UpdateWindow")
	pGetMsg           = user32.NewProc("GetMessageW")
	pTranslate        = user32.NewProc("TranslateMessage")
	pDispatch         = user32.NewProc("DispatchMessageW")
	pPostQuit         = user32.NewProc("PostQuitMessage")
	pSend             = user32.NewProc("SendMessageW")
	pMove             = user32.NewProc("MoveWindow")
	pSetText          = user32.NewProc("SetWindowTextW")
	pGetText          = user32.NewProc("GetWindowTextW")
	pFocus            = user32.NewProc("SetFocus")
	pDestroy          = user32.NewProc("DestroyWindow")
	pEnable           = user32.NewProc("EnableWindow")
	pPostMessage      = user32.NewProc("PostMessageW")
	pInvalidate       = user32.NewProc("InvalidateRect")
	pBeginPaint       = user32.NewProc("BeginPaint")
	pEndPaint         = user32.NewProc("EndPaint")
	pGetClient        = user32.NewProc("GetClientRect")
	pGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	pMessageBox       = user32.NewProc("MessageBoxW")
	pLoadIcon         = user32.NewProc("LoadIconW")
	pDrawIconEx       = user32.NewProc("DrawIconEx")
	pSetCursor        = user32.NewProc("SetCursor")
	pLoadCursor       = user32.NewProc("LoadCursorW")
	pFillRect         = user32.NewProc("FillRect")
	pDrawText         = user32.NewProc("DrawTextW")
	pCreateFont       = gdi32.NewProc("CreateFontW")
	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pSetTextColor     = gdi32.NewProc("SetTextColor")
	pCreateBrush      = gdi32.NewProc("CreateSolidBrush")
	pCreatePen        = gdi32.NewProc("CreatePen")
	pSelect           = gdi32.NewProc("SelectObject")
	pDelete           = gdi32.NewProc("DeleteObject")
	pRoundRect        = gdi32.NewProc("RoundRect")
	pRectangle        = gdi32.NewProc("Rectangle")
	pEllipse          = gdi32.NewProc("Ellipse")
	pMoveTo           = gdi32.NewProc("MoveToEx")
	pLineTo           = gdi32.NewProc("LineTo")
	pPolygon          = gdi32.NewProc("Polygon")
	pGetStock         = gdi32.NewProc("GetStockObject")
	pCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pBitBlt           = gdi32.NewProc("BitBlt")
	pDeleteDC         = gdi32.NewProc("DeleteDC")
	pDragAccept       = shell32.NewProc("DragAcceptFiles")
	pDragQuery        = shell32.NewProc("DragQueryFileW")
	pDragFinish       = shell32.NewProc("DragFinish")
	pBrowse           = shell32.NewProc("SHBrowseForFolderW")
	pPathFromID       = shell32.NewProc("SHGetPathFromIDListW")
	pCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	pCoInitialize     = ole32.NewProc("CoInitializeEx")
	pCoUninitialize   = ole32.NewProc("CoUninitialize")
	pCoCreate         = ole32.NewProc("CoCreateInstance")
)

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x00010000
	WM_CREATE           = 1
	WM_DESTROY          = 2
	WM_SIZE             = 5
	WM_PAINT            = 15
	WM_CLOSE            = 16
	WM_COMMAND          = 0x111
	WM_LBUTTONDOWN      = 0x201
	WM_LBUTTONUP        = 0x202
	WM_MOUSEMOVE        = 0x200
	WM_MOUSEWHEEL       = 0x20A
	WM_DROPFILES        = 0x233
	WM_APP_RESULT       = 0x8001
	WM_APP_DONE         = 0x8002
	WM_APP_CONV_TICK    = 0x8003
	WM_APP_CONV_DONE    = 0x8004
	WM_SETFONT          = 0x30
	WM_GETMINMAXINFO    = 0x24
	DT_CENTER           = 1
	DT_RIGHT            = 2
	DT_VCENTER          = 4
	DT_SINGLELINE       = 0x20
	DT_END_ELLIPSIS     = 0x8000
	TRANSPARENT         = 1
	PS_SOLID            = 0
	PS_DOT              = 2
	NULL_BRUSH          = 5
	SRCCOPY             = 0xCC0020
	MB_OK               = 0
)

func w(s string) *uint16          { return syscall.StringToUTF16Ptr(s) }
func ptr(s string) uintptr        { return uintptr(unsafe.Pointer(w(s))) }
func color(r, g, b uint8) uintptr { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }

var (
	cNavy   = color(23, 42, 64)
	cNavy2  = color(33, 57, 85)
	cBlue   = color(36, 102, 210)
	cBg     = color(243, 246, 250)
	cWhite  = color(255, 255, 255)
	cInk    = color(24, 42, 66)
	cMuted  = color(92, 111, 136)
	cBorder = color(186, 199, 216)
	cSoft   = color(237, 243, 250)
	cGreen  = color(25, 137, 101)
	cYellow = color(174, 115, 24)
	cRed    = color(205, 58, 75)
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type wc struct {
	Style                                                   uint32
	WndProc                                                 uintptr
	ClsExtra, WndExtra                                      int32
	Instance, Icon, Cursor, Background, MenuName, ClassName uintptr
}
type ps struct {
	Hdc                uintptr
	Erase              int32
	RcPaint            rect
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type browseinfo struct {
	Owner, Root, DisplayName, Title uintptr
	Flags                           uint32
	Callback, Param                 uintptr
	Image                           int32
}
type guid struct {
	D1     uint32
	D2, D3 uint16
	D4     [8]byte
}
type minmaxinfo struct{ Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize point }
type pdfEntry struct {
	Name, Path, Status string
	RGB, CMYK          bool
	Size               int64
	Pages              int
	Failure            string
}
type button struct {
	id         int
	name, icon string
	box        rect
	fill, text uintptr
	enabled    bool
}

var (
	mainH, saveH, saveFolderH, saveNameH                                       uintptr
	iconH, fontRegular, fontSmall, fontBold, fontTitle, fontSubtitle, fontTiny uintptr
	entries                                                                    []pdfEntry
	pending                                                                    []string
	seen                                                                       = map[string]bool{}
	resultMu                                                                   sync.Mutex
	results                                                                    []pdfEntry
	running                                                                    bool
	converting                                                                 bool
	convDone, convTotal, convOK                                                int
	convFolder                                                                 string
	convErrors                                                                 []string
	convMutex                                                                  sync.Mutex
	done, total                                                                int
	width, height                                                              int32 = 1160, 760
	selected, offset, hover                                                    int   = -1, 0, -1
	buttons                                                                    []button
)

func rectOf(x, y, cx, cy int32) rect { return rect{x, y, x + cx, y + cy} }
func inside(r rect, x, y int32) bool { return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom }
func hresult(r uintptr) bool         { return int32(r) >= 0 }
func callCOM(obj uintptr, index uintptr, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	f := *(*uintptr)(unsafe.Pointer(vtbl + index*unsafe.Sizeof(uintptr(0))))
	all := append([]uintptr{obj}, args...)
	r, _, _ := syscall.SyscallN(f, all...)
	return r
}
func openPDFDialog() ([]string, error) {
	clsid := guid{0xdc1c5a9c, 0xe88a, 0x4dde, [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
	iid := guid{0xd57c7288, 0xd4ad, 0x4768, [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
	var dialog uintptr
	hr, _, _ := pCoCreate.Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&dialog)))
	if !hresult(hr) || dialog == 0 {
		return nil, fmt.Errorf("диалог выбора файлов недоступен (0x%X)", uint32(hr))
	}
	defer callCOM(dialog, 2)
	// FOS_FORCEFILESYSTEM | FOS_ALLOWMULTISELECT | FOS_FILEMUSTEXIST
	var options uintptr
	callCOM(dialog, 10, uintptr(unsafe.Pointer(&options)))
	callCOM(dialog, 9, options|0x40|0x200|0x1000)
	title := w("Выберите PDF-файлы")
	callCOM(dialog, 17, uintptr(unsafe.Pointer(title)))
	type filter struct{ Label, Pattern uintptr }
	label, spec := w("PDF-документы (*.pdf)"), w("*.pdf")
	filters := []filter{{uintptr(unsafe.Pointer(label)), uintptr(unsafe.Pointer(spec))}}
	callCOM(dialog, 4, 1, uintptr(unsafe.Pointer(&filters[0])))
	hr = callCOM(dialog, 3, mainH)
	runtime.KeepAlive(title)
	runtime.KeepAlive(label)
	runtime.KeepAlive(spec)
	runtime.KeepAlive(filters)
	if uint32(hr) == 0x800704C7 {
		return nil, nil
	} // cancelled
	if !hresult(hr) {
		return nil, fmt.Errorf("не удалось открыть диалог (0x%X)", uint32(hr))
	}
	var items uintptr
	hr = callCOM(dialog, 27, uintptr(unsafe.Pointer(&items)))
	if !hresult(hr) || items == 0 {
		return nil, fmt.Errorf("не удалось получить выбранные файлы")
	}
	defer callCOM(items, 2)
	var count uint32
	callCOM(items, 7, uintptr(unsafe.Pointer(&count)))
	var paths []string
	for i := uint32(0); i < count; i++ {
		var item uintptr
		if !hresult(callCOM(items, 8, uintptr(i), uintptr(unsafe.Pointer(&item)))) || item == 0 {
			continue
		}
		var name uintptr
		hr = callCOM(item, 5, 0x80058000, uintptr(unsafe.Pointer(&name))) // SIGDN_FILESYSPATH
		if hresult(hr) && name != 0 {
			p := utf16PtrToString(name)
			if strings.EqualFold(filepath.Ext(p), ".pdf") {
				paths = append(paths, p)
			}
			pCoTaskMemFree.Call(name)
		}
		callCOM(item, 2)
	}
	return paths, nil
}
func utf16PtrToString(p uintptr) string {
	if p == 0 {
		return ""
	}
	var buf []uint16
	for i := uintptr(0); i < 32768; i++ {
		c := *(*uint16)(unsafe.Pointer(p + i*2))
		if c == 0 {
			break
		}
		buf = append(buf, c)
	}
	return syscall.UTF16ToString(buf)
}
func chooseFolder(owner uintptr) string {
	display := make([]uint16, 260)
	title := w("Выберите папку с PDF-файлами")
	bi := browseinfo{Owner: owner, DisplayName: uintptr(unsafe.Pointer(&display[0])), Title: uintptr(unsafe.Pointer(title)), Flags: 0x40 | 0x1}
	pidl, _, _ := pBrowse.Call(uintptr(unsafe.Pointer(&bi)))
	runtime.KeepAlive(display)
	runtime.KeepAlive(title)
	if pidl == 0 {
		return ""
	}
	defer pCoTaskMemFree.Call(pidl)
	buf := make([]uint16, 32768)
	ok, _, _ := pPathFromID.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
func alert(text string)           { pMessageBox.Call(mainH, ptr(text), ptr("PDF Color Checker"), 0x40) }
func setText(h uintptr, s string) { pSetText.Call(h, ptr(s)) }
func getText(h uintptr) string {
	u := make([]uint16, 32768)
	n, _, _ := pGetText.Call(h, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)))
	return syscall.UTF16ToString(u[:n])
}
func makeChild(cls, text string, style uint32, x, y, cx, cy int32, parent uintptr, id int) uintptr {
	h, _, _ := pCreateWindow.Call(0, ptr(cls), ptr(text), uintptr(style), uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), parent, uintptr(id), 0, 0)
	if h != 0 && fontRegular != 0 {
		pSend.Call(h, WM_SETFONT, fontRegular, 1)
	}
	return h
}
func loadFont(size, weight int32) uintptr {
	f, _, _ := pCreateFont.Call(uintptr(size), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, ptr("Segoe UI"))
	return f
}
func fill(h uintptr, r rect, c uintptr) {
	b, _, _ := pCreateBrush.Call(c)
	pFillRect.Call(h, uintptr(unsafe.Pointer(&r)), b)
	pDelete.Call(b)
}
func round(h uintptr, r rect, rad int32, bg, border uintptr) {
	br, _, _ := pCreateBrush.Call(bg)
	pen, _, _ := pCreatePen.Call(PS_SOLID, 1, border)
	oldbr, _, _ := pSelect.Call(h, br)
	oldpen, _, _ := pSelect.Call(h, pen)
	pRoundRect.Call(h, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(rad), uintptr(rad))
	pSelect.Call(h, oldbr)
	pSelect.Call(h, oldpen)
	pDelete.Call(br)
	pDelete.Call(pen)
}
func outline(h uintptr, r rect, rad int32, clr uintptr, dashed bool) {
	style := uintptr(0)
	if dashed {
		style = PS_DOT
	}
	pen, _, _ := pCreatePen.Call(style, 1, clr)
	oldp, _, _ := pSelect.Call(h, pen)
	empty, _, _ := pGetStock.Call(NULL_BRUSH)
	oldb, _, _ := pSelect.Call(h, empty)
	pRoundRect.Call(h, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(rad), uintptr(rad))
	pSelect.Call(h, oldb)
	pSelect.Call(h, oldp)
	pDelete.Call(pen)
}
func line(h uintptr, x1, y1, x2, y2 int32, clr uintptr, stroke int32) {
	pen, _, _ := pCreatePen.Call(0, uintptr(stroke), clr)
	old, _, _ := pSelect.Call(h, pen)
	pMoveTo.Call(h, uintptr(x1), uintptr(y1), 0)
	pLineTo.Call(h, uintptr(x2), uintptr(y2))
	pSelect.Call(h, old)
	pDelete.Call(pen)
}
func ellipse(h uintptr, r rect, fg, border uintptr) {
	br, _, _ := pCreateBrush.Call(fg)
	pen, _, _ := pCreatePen.Call(0, 1, border)
	ob, _, _ := pSelect.Call(h, br)
	op, _, _ := pSelect.Call(h, pen)
	pEllipse.Call(h, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom))
	pSelect.Call(h, ob)
	pSelect.Call(h, op)
	pDelete.Call(br)
	pDelete.Call(pen)
}
func drawText(h uintptr, t string, r rect, c, f uintptr, flags uintptr) {
	if f != 0 {
		pSelect.Call(h, f)
	}
	pSetBkMode.Call(h, TRANSPARENT)
	pSetTextColor.Call(h, c)
	pDrawText.Call(h, ptr(t), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_VCENTER|flags)
}
func iconDraw(h uintptr, typ string, x, y int32, clr uintptr) {
	// Purpose-built vector icons, rendered in any Windows version without icon-font dependencies.
	switch typ {
	case "file":
		outline(h, rectOf(x+3, y+1, 14, 18), 3, clr, false)
		line(h, x+6, y+9, x+14, y+9, clr, 1)
		line(h, x+6, y+13, x+13, y+13, clr, 1)
	case "folder":
		outline(h, rectOf(x+1, y+6, 20, 13), 4, clr, false)
		line(h, x+3, y+6, x+3, y+3, clr, 2)
		line(h, x+3, y+3, x+10, y+3, clr, 2)
		line(h, x+10, y+3, x+13, y+6, clr, 2)
	case "play":
		pts := [3]point{{x + 6, y + 3}, {x + 6, y + 19}, {x + 19, y + 11}}
		br, _, _ := pCreateBrush.Call(clr)
		pen, _, _ := pCreatePen.Call(0, 1, clr)
		ob, _, _ := pSelect.Call(h, br)
		op, _, _ := pSelect.Call(h, pen)
		pPolygon.Call(h, uintptr(unsafe.Pointer(&pts[0])), 3)
		pSelect.Call(h, ob)
		pSelect.Call(h, op)
		pDelete.Call(br)
		pDelete.Call(pen)
	case "convert":
		ellipse(h, rectOf(x+1, y+5, 11, 11), color(255, 255, 255), clr)
		line(h, x+11, y+10, x+21, y+10, clr, 2)
		line(h, x+17, y+6, x+21, y+10, clr, 2)
		line(h, x+21, y+10, x+17, y+14, clr, 2)
	case "download":
		line(h, x+11, y+1, x+11, y+13, clr, 2)
		line(h, x+6, y+9, x+11, y+14, clr, 2)
		line(h, x+11, y+14, x+16, y+9, clr, 2)
		line(h, x+3, y+16, x+3, y+20, clr, 2)
		line(h, x+3, y+20, x+19, y+20, clr, 2)
		line(h, x+19, y+20, x+19, y+16, clr, 2)
	case "trash":
		outline(h, rectOf(x+6, y+7, 11, 13), 2, clr, false)
		line(h, x+4, y+5, x+19, y+5, clr, 2)
		line(h, x+9, y+2, x+14, y+2, clr, 2)
		line(h, x+10, y+10, x+10, y+17, clr, 1)
		line(h, x+14, y+10, x+14, y+17, clr, 1)
	case "plus":
		line(h, x+11, y+3, x+11, y+19, clr, 2)
		line(h, x+3, y+11, x+19, y+11, clr, 2)
	case "cloud":
		ellipse(h, rectOf(x+6, y+9, 15, 13), cWhite, clr)
		ellipse(h, rectOf(x+14, y+4, 20, 17), cWhite, clr)
		fill(h, rectOf(x+6, y+13, 30, 10), cWhite)
		line(h, x+6, y+24, x+36, y+24, clr, 2)
		line(h, x+22, y+12, x+22, y+29, clr, 2)
		line(h, x+16, y+23, x+22, y+29, clr, 2)
		line(h, x+22, y+29, x+28, y+23, clr, 2)
	case "info":
		ellipse(h, rectOf(x+1, y+1, 20, 20), cWhite, clr)
		line(h, x+11, y+9, x+11, y+16, clr, 2)
		ellipse(h, rectOf(x+10, y+5, 3, 3), clr, clr)
	}
}

func layoutButtons() {
	x := int32(28)
	y := int32(148)
	h := int32(45)
	specs := []struct {
		id         int
		name, icon string
		wid        int32
		bg, fg     uintptr
	}{
		{1, "Добавить файлы", "file", 178, cBlue, cWhite},
		{2, "Добавить папку", "folder", 172, cWhite, cInk},
		{3, "Проверить", "play", 131, cWhite, cInk},
		{6, "В CMYK FOGRA39", "convert", 207, cNavy2, cWhite},
		{4, "Выгрузить TXT", "download", 169, cGreen, cWhite},
		{5, "Очистить", "trash", 133, cWhite, cInk},
	}
	usable := width - 56
	totalW := int32(0)
	for _, s := range specs {
		totalW += s.wid
	}
	totalW += int32(len(specs)-1) * 9
	scale := float64(1)
	if totalW > usable {
		scale = float64(usable) / float64(totalW)
	}
	buttons = buttons[:0]
	for _, s := range specs {
		bw := int32(float64(s.wid) * scale)
		if bw < 108 {
			bw = 108
		}
		buttons = append(buttons, button{id: s.id, name: s.name, icon: s.icon, box: rectOf(x, y, bw, h), fill: s.bg, text: s.fg, enabled: (!running && !converting) || s.id == 4})
		x += bw + int32(9*scale)
	}
}
func progressCounts() (green, red, mixed, unknown int) {
	for _, e := range entries {
		switch e.Status {
		case "CMYK":
			green++
		case "RGB":
			red++
		case "CMYK + RGB":
			mixed++
		default:
			unknown++
		}
	}
	return
}
func renderButton(h uintptr, b button) {
	bg, fg := b.fill, b.text
	if !b.enabled {
		bg = color(232, 236, 243)
		fg = color(153, 164, 179)
	} else if hover == b.id {
		switch b.id {
		case 1:
			bg = color(22, 84, 186)
		case 4:
			bg = color(16, 112, 82)
		case 6:
			bg = color(43, 76, 113)
		default:
			bg = color(237, 243, 251)
		}
	}
	edge := bg
	if b.id != 1 && b.id != 4 && b.id != 6 {
		edge = cBorder
	}
	round(h, b.box, 11, bg, edge)
	iconX := b.box.Left + 16
	if b.box.Right-b.box.Left < 145 {
		iconX += 1
	}
	iconDraw(h, b.icon, iconX, b.box.Top+11, fg)
	drawText(h, b.name, rect{iconX + 30, b.box.Top + 1, b.box.Right - 7, b.box.Bottom - 1}, fg, fontBold, 0)
}
func paintMain(hwnd uintptr) {
	var p ps
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&p)))
	if hdc == 0 {
		return
	}
	defer pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&p)))
	client := rect{}
	pGetClient.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	ww, hh := client.Right, client.Bottom
	if ww < 800 || hh < 560 {
		return
	}
	width, height = ww, hh
	layoutButtons()
	mem, _, _ := pCompatibleDC.Call(hdc)
	bm, _, _ := pCompatibleBitmap.Call(hdc, uintptr(ww), uintptr(hh))
	oldBM, _, _ := pSelect.Call(mem, bm)
	defer func() { pSelect.Call(mem, oldBM); pDelete.Call(bm); pDeleteDC.Call(mem) }()
	fill(mem, rect{0, 0, ww, hh}, cBg)
	// Header with restrained tonal band and crisp divider
	fill(mem, rect{0, 0, ww, 128}, cNavy)
	fill(mem, rect{0, 126, ww, 128}, cBlue)
	round(mem, rectOf(28, 23, 77, 77), 16, cNavy2, cNavy2)
	if iconH != 0 {
		pDrawIconEx.Call(mem, 40, 34, iconH, 52, 52, 0, 0, 3)
	} else {
		round(mem, rectOf(42, 35, 48, 51), 8, cWhite, cWhite)
		fill(mem, rectOf(51, 44, 11, 11), cBlue)
		fill(mem, rectOf(66, 44, 11, 11), color(236, 190, 35))
		fill(mem, rectOf(51, 59, 11, 11), color(218, 51, 105))
		fill(mem, rectOf(66, 59, 11, 11), cInk)
	}
	drawText(mem, "PDF COLOR CHECKER", rectOf(122, 26, ww-390, 46), cWhite, fontTitle, 0)
	drawText(mem, "Проверка цветовых пространств PDF  ·  Быстрая допечатная проверка", rectOf(124, 76, ww-340, 33), color(197, 216, 238), fontRegular, 0)
	round(mem, rectOf(ww-257, 36, 74, 33), 17, cNavy2, color(67, 95, 130))
	drawText(mem, "v5.4", rectOf(ww-254, 36, 68, 31), cWhite, fontSmall, DT_CENTER)
	round(mem, rectOf(ww-174, 35, 147, 34), 16, cNavy2, color(77, 106, 141))
	iconDraw(mem, "info", ww-162, 42, color(220, 235, 251))
	drawText(mem, "О программе", rectOf(ww-134, 36, 102, 32), cWhite, fontSmall, 0)
	// Toolbar
	for _, b := range buttons {
		renderButton(mem, b)
	}
	// cards: drag target & processing
	margin := int32(28)
	panelY := int32(210)
	panelH := int32(113)
	leftW := (ww - 3*margin - 16) * 52 / 100
	dr := rectOf(margin, panelY, leftW, panelH)
	round(mem, dr, 16, cWhite, cBorder)
	line(mem, dr.Left+1, dr.Bottom-1, dr.Right-1, dr.Bottom-1, cBorder, 1)
	outline(mem, rect{dr.Left + 5, dr.Top + 5, dr.Right - 5, dr.Bottom - 5}, 14, color(154, 183, 221), true)
	iconDraw(mem, "cloud", dr.Left+23, dr.Top+31, color(78, 124, 180))
	drawText(mem, "ПЕРЕТАЩИТЕ PDF-ФАЙЛЫ СЮДА", rectOf(dr.Left+80, dr.Top+19, dr.Right-dr.Left-92, 30), cInk, fontBold, 0)
	drawText(mem, "Или выберите файлы / папку кнопками выше", rectOf(dr.Left+80, dr.Top+53, dr.Right-dr.Left-92, 29), cMuted, fontRegular, 0)
	rightX := dr.Right + 16
	pr := rect{rightX, panelY, ww - margin, panelY + panelH}
	round(mem, pr, 16, cWhite, cBorder)
	line(mem, pr.Left+1, pr.Bottom-1, pr.Right-1, pr.Bottom-1, cBorder, 1)
	drawText(mem, "ХОД ПРОВЕРКИ", rectOf(rightX+19, panelY+11, pr.Right-rightX-98, 30), cInk, fontBold, 0)
	percent := 0
	if converting && convTotal > 0 {
		percent = convDone * 100 / convTotal
	} else if total > 0 {
		percent = done * 100 / total
	} else if len(entries) > 0 {
		percent = 100
	}
	if percent > 100 {
		percent = 100
	}
	drawText(mem, fmt.Sprintf("%d%%", percent), rectOf(pr.Right-83, panelY+13, 62, 27), cBlue, fontBold, DT_RIGHT)
	bar := rect{rightX + 19, panelY + 49, pr.Right - 19, panelY + 61}
	round(mem, bar, 10, color(231, 237, 246), color(231, 237, 246))
	if percent > 0 {
		end := bar.Left + (bar.Right-bar.Left)*int32(percent)/100
		if end < bar.Left+11 {
			end = bar.Left + 11
		}
		round(mem, rect{bar.Left, bar.Top, end, bar.Bottom}, 10, cGreen, cGreen)
	}
	progressLabel := "Ожидание файлов"
	if running {
		progressLabel = fmt.Sprintf("Проверено %d из %d файлов", done, total)
	} else if len(entries) > 0 {
		progressLabel = fmt.Sprintf("Готово. Проверено файлов: %d", len(entries))
	}
	drawText(mem, progressLabel, rectOf(rightX+19, panelY+69, pr.Right-rightX-39, 28), cMuted, fontRegular, 0)
	// compact counters
	green, red, mixed, unknown := progressCounts()
	rowY := int32(334)
	totalText := fmt.Sprintf("ВСЕГО  %d", len(entries))
	groups := []struct {
		text   string
		bg, fg uintptr
	}{
		{totalText, color(232, 239, 249), color(63, 97, 143)},
		{fmt.Sprintf("CMYK  %d", green), color(228, 247, 237), color(34, 137, 78)},
		{fmt.Sprintf("RGB  %d", red), color(255, 232, 235), color(201, 50, 65)},
		{fmt.Sprintf("СМЕШАННЫЕ  %d", mixed), color(255, 245, 219), color(161, 103, 29)},
		{fmt.Sprintf("ОШИБКИ  %d", unknown), color(236, 240, 245), color(94, 109, 127)},
	}
	xx := margin
	for i, g := range groups {
		bw := int32(111)
		if i == 3 {
			bw = 167
		}
		if i == 4 {
			bw = 138
		}
		if i == 0 {
			bw = 128
		}
		round(mem, rectOf(xx, rowY, bw, 36), 12, g.bg, cBorder)
		drawText(mem, g.text, rectOf(xx+10, rowY+2, bw-20, 31), g.fg, fontSmall, DT_CENTER)
		xx += bw + 9
	}
	// table card: header, rows, fixed viewport
	tx := margin
	ty := int32(380)
	tw := ww - 2*margin
	tableBottom := hh - 58
	if tableBottom < ty+68 {
		tableBottom = ty + 68
	}
	round(mem, rect{tx, ty, tx + tw, tableBottom}, 15, cWhite, cBorder)
	fill(mem, rect{tx + 1, ty + 1, tx + tw - 1, ty + 43}, color(231, 237, 246))
	colName := tx + int32(float64(tw)*0.070)
	colModel := tx + int32(float64(tw)*0.602)
	colSize := tx + int32(float64(tw)*0.752)
	colPages := tx + int32(float64(tw)*0.837)
	colStatus := tx + int32(float64(tw)*0.918)
	drawText(mem, "№", rectOf(tx+17, ty+6, 34, 31), cInk, fontBold, 0)
	drawText(mem, "НАЗВАНИЕ ФАЙЛА", rect{colName, ty + 6, colModel - 8, ty + 37}, cInk, fontBold, 0)
	drawText(mem, "МОДЕЛЬ", rect{colModel, ty + 6, colSize - 8, ty + 37}, cInk, fontBold, 0)
	drawText(mem, "РАЗМЕР", rect{colSize, ty + 6, colPages - 3, ty + 37}, cInk, fontBold, 0)
	drawText(mem, "СТР.", rect{colPages, ty + 6, colStatus - 4, ty + 37}, cInk, fontBold, 0)
	drawText(mem, "СТАТУС", rect{colStatus, ty + 6, tx + tw - 15, ty + 37}, cInk, fontBold, 0)
	line(mem, tx+1, ty+43, tx+tw-1, ty+43, cBorder, 1)
	outline(mem, rect{tx, ty, tx + tw, tableBottom}, 15, cBorder, false)
	rowH := int32(38)
	visible := int((tableBottom - ty - 45) / rowH)
	if visible < 1 {
		visible = 1
	}
	if offset > maxInt(0, len(entries)-visible) {
		offset = maxInt(0, len(entries)-visible)
	}
	for i := 0; i < visible && offset+i < len(entries); i++ {
		ix := offset + i
		e := entries[ix]
		y := ty + 44 + int32(i)*rowH
		if y+rowH > tableBottom {
			break
		}
		bg := cWhite
		fg := cInk
		statusColor := cMuted
		label := "НЕ ОПР."
		switch e.Status {
		case "CMYK":
			bg = color(237, 250, 242)
			fg = color(30, 99, 62)
			statusColor = cGreen
			label = "OK"
		case "RGB":
			bg = color(255, 235, 238)
			fg = color(166, 41, 60)
			statusColor = cRed
			label = "RGB"
		case "CMYK + RGB":
			bg = color(255, 247, 222)
			fg = color(132, 90, 30)
			statusColor = cYellow
			label = "СМЕШАН"
		default:
			bg = color(240, 244, 249)
			fg = color(88, 105, 125)
			statusColor = cMuted
		}
		if ix == selected {
			bg = color(225, 239, 255)
		}
		fill(mem, rect{tx + 1, y, tx + tw - 1, y + rowH}, bg)
		drawText(mem, fmt.Sprintf("%d", ix+1), rectOf(tx+18, y, 36, rowH), fg, fontSmall, 0)
		drawText(mem, e.Name, rect{colName, y + 1, colModel - 14, y + rowH - 1}, fg, fontSmall, DT_END_ELLIPSIS)
		drawText(mem, e.Status, rect{colModel, y + 1, colSize - 10, y + rowH - 1}, fg, fontSmall, DT_END_ELLIPSIS)
		drawText(mem, formatSize(e.Size), rect{colSize, y + 1, colPages - 8, y + rowH - 1}, fg, fontSmall, 0)
		pages := "—"
		if e.Pages > 0 {
			pages = strconv.Itoa(e.Pages)
		}
		drawText(mem, pages, rect{colPages, y + 1, colStatus - 6, y + rowH - 1}, fg, fontSmall, 0)
		ellipse(mem, rectOf(colStatus, y+14, 10, 10), statusColor, statusColor)
		drawText(mem, label, rect{colStatus + 17, y + 2, tx + tw - 10, y + rowH - 2}, fg, fontTiny, DT_END_ELLIPSIS)
		line(mem, tx+1, y+rowH-1, tx+tw-1, y+rowH-1, color(238, 241, 245), 1)
	}
	if len(entries) == 0 {
		iconDraw(mem, "file", tx+tw/2-10, ty+91, color(166, 182, 202))
		drawText(mem, "Здесь появятся результаты проверки", rectOf(tx+tw/2-230, ty+126, 460, 38), cMuted, fontRegular, DT_CENTER)
		drawText(mem, "Добавьте PDF-файлы, чтобы начать", rectOf(tx+tw/2-230, ty+160, 460, 27), color(157, 172, 190), fontSmall, DT_CENTER)
	}
	if len(entries) > visible {
		track := rect{tx + tw - 8, ty + 47, tx + tw - 4, tableBottom - 8}
		round(mem, track, 4, color(236, 240, 246), color(236, 240, 246))
		thumbHeight := int32(float64(track.Bottom-track.Top) * float64(visible) / float64(len(entries)))
		if thumbHeight < 24 {
			thumbHeight = 24
		}
		top := track.Top + int32(float64(track.Bottom-track.Top-thumbHeight)*float64(offset)/float64(maxInt(1, len(entries)-visible)))
		round(mem, rect{track.Left, top, track.Right, top + thumbHeight}, 4, color(147, 167, 193), color(147, 167, 193))
	}
	// footer
	fill(mem, rect{0, hh - 43, ww, hh}, color(229, 236, 245))
	line(mem, 0, hh-43, ww, hh-43, cBorder, 1)
	foot := "Готово к проверке PDF"
	if converting {
		foot = fmt.Sprintf("Конвертация FOGRA39: %d из %d", convDone, convTotal)
	} else if running {
		foot = fmt.Sprintf("Проверка: %d из %d  •  окно остаётся активным", done, total)
	} else if len(entries) > 0 {
		foot = fmt.Sprintf("Готово  •  Проверено: %d  •  RGB или ошибка: %d", len(entries), red+mixed+unknown)
	}
	drawText(mem, foot, rectOf(28, hh-39, ww-190, 32), cMuted, fontSmall, 0)
	drawText(mem, "PDF Color Checker 5.4", rectOf(ww-220, hh-39, 190, 32), cMuted, fontTiny, DT_RIGHT)
	pBitBlt.Call(hdc, 0, 0, uintptr(ww), uintptr(hh), mem, 0, 0, SRCCOPY)
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func formatSize(n int64) string {
	if n <= 0 {
		return "—"
	}
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f МБ", float64(n)/(1<<20))
	}
	if n >= 1<<10 {
		return fmt.Sprintf("%.0f КБ", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d Б", n)
}
func repaint() {
	if mainH != 0 {
		pInvalidate.Call(mainH, 0, 0)
	}
}

func enqueue(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		_ = filepath.WalkDir(path, func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if e.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(p), ".pdf") {
				queueFile(p)
			}
			return nil
		})
		return
	}
	queueFile(path)
}
func queueFile(path string) {
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	key := strings.ToLower(abs)
	if seen[key] {
		return
	}
	seen[key] = true
	pending = append(pending, abs)
}
func launch() {
	if running || converting || len(pending) == 0 {
		return
	}
	tasks := append([]string(nil), pending...)
	pending = nil
	running = true
	done = 0
	total = len(tasks)
	repaint()
	go func() {
		workers := runtime.NumCPU()
		if workers > 4 {
			workers = 4
		}
		if workers < 1 {
			workers = 1
		}
		jobs := make(chan string)
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for p := range jobs {
					entry := checkPDF(p)
					resultMu.Lock()
					results = append(results, entry)
					resultMu.Unlock()
					pPostMessage.Call(mainH, WM_APP_RESULT, 0, 0)
				}
			}()
		}
		for _, p := range tasks {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		pPostMessage.Call(mainH, WM_APP_DONE, 0, 0)
	}()
}
func drain() {
	resultMu.Lock()
	batch := results
	results = nil
	resultMu.Unlock()
	if len(batch) == 0 {
		return
	}
	entries = append(entries, batch...)
	done += len(batch)
	repaint()
}
func exportLines() []string {
	var output []string
	for _, e := range entries {
		if e.Status == "CMYK" {
			continue
		}
		line := e.Name
		if e.Status != "RGB" && e.Status != "CMYK + RGB" {
			reason := e.Failure
			if reason == "" {
				reason = "цветовая модель не определена"
			}
			line += " — ОШИБКА: " + reason
		}
		output = append(output, line)
	}
	return output
}
func showSaveWindow() {
	if running || converting {
		alert("Дождитесь окончания обработки перед выгрузкой.")
		return
	}
	if len(exportLines()) == 0 {
		alert("Нет файлов с RGB или ошибками для выгрузки.")
		return
	}
	if saveH != 0 {
		pFocus.Call(saveH)
		return
	}
	h, _, _ := pCreateWindow.Call(0, ptr("PDFCheckerSave5"), ptr("Выгрузить TXT · PDF Color Checker"), WS_OVERLAPPEDWINDOW|WS_VISIBLE,
		350, 240, 600, 335, mainH, 0, 0, 0)
	if h == 0 {
		alert("Не удалось открыть окно сохранения.")
		return
	}
	saveH = h
	makeChild("STATIC", "Папка для сохранения", WS_CHILD|WS_VISIBLE, 25, 22, 500, 24, h, 0)
	home, _ := os.UserHomeDir()
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		_ = d
	}
	saveFolderH = makeChild("EDIT", home, WS_CHILD|WS_VISIBLE|WS_TABSTOP|0x00800000|0x200, 25, 51, 412, 30, h, 301)
	makeChild("BUTTON", "Обзор…", WS_CHILD|WS_VISIBLE|WS_TABSTOP, 450, 51, 113, 31, h, 302)
	makeChild("STATIC", "Имя TXT-файла", WS_CHILD|WS_VISIBLE, 25, 100, 500, 24, h, 0)
	saveNameH = makeChild("EDIT", "PDF_ошибки.txt", WS_CHILD|WS_VISIBLE|WS_TABSTOP|0x00800000|0x200, 25, 129, 538, 31, h, 303)
	makeChild("STATIC", "Список: RGB, смешанные и неопределённые файлы с пометкой об ошибке", WS_CHILD|WS_VISIBLE, 25, 174, 549, 27, h, 0)
	makeChild("BUTTON", "Сохранить TXT", WS_CHILD|WS_VISIBLE|WS_TABSTOP, 296, 214, 153, 36, h, 304)
	makeChild("BUTTON", "Отмена", WS_CHILD|WS_VISIBLE|WS_TABSTOP, 463, 214, 102, 36, h, 305)
	pFocus.Call(saveNameH)
}
func doSave() {
	folder, filename := strings.TrimSpace(getText(saveFolderH)), strings.TrimSpace(getText(saveNameH))
	if folder == "" || filename == "" {
		alert("Укажите папку и название файла.")
		return
	}
	if filepath.Base(filename) != filename || filename == "." || filename == ".." {
		alert("Введите название файла отдельно от пути.")
		return
	}
	if filepath.Ext(filename) == "" {
		filename += ".txt"
	}
	if !strings.EqualFold(filepath.Ext(filename), ".txt") {
		alert("Требуется расширение TXT.")
		return
	}
	st, err := os.Stat(folder)
	if err != nil || !st.IsDir() {
		alert("Папка не найдена.")
		return
	}
	path := filepath.Join(folder, filename)
	if _, err = os.Stat(path); err == nil {
		choice, _, _ := pMessageBox.Call(saveH, ptr("Файл уже существует. Перезаписать?"), ptr("Подтверждение"), 0x24)
		if choice != 6 {
			return
		}
	}
	lines := exportLines()
	payload := []byte("\xef\xbb\xbf" + strings.Join(lines, "\r\n") + "\r\n")
	if err = os.WriteFile(path, payload, 0644); err != nil {
		alert("Не удалось сохранить TXT: " + err.Error())
		return
	}
	pDestroy.Call(saveH)
	alert(fmt.Sprintf("Сохранено %d названий файлов.\n%s", len(lines), path))
}
func saveProc(hwnd uintptr, m uint32, wp, lp uintptr) uintptr {
	switch m {
	case WM_COMMAND:
		switch int(wp & 0xffff) {
		case 302:
			if p := chooseFolder(hwnd); p != "" {
				setText(saveFolderH, p)
			}
		case 304:
			doSave()
		case 305:
			pDestroy.Call(hwnd)
		}
		return 0
	case WM_CLOSE:
		pDestroy.Call(hwnd)
		return 0
	case WM_DESTROY:
		saveH = 0
		saveFolderH = 0
		saveNameH = 0
		return 0
	}
	r, _, _ := pDefWindow.Call(hwnd, uintptr(m), wp, lp)
	return r
}

// FOGRA39 conversion uses the Ghostscript runtime that is bundled inside this EXE.
// The Windows build embeds the official runtime and the ECI ISO Coated v2 ICC profile.
// No separate Ghostscript installation is needed on the end-user computer.
func validFOGRA(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 128 {
		return false
	}
	// ICC header: channel signature at bytes 16..20; required CMYK.
	return string(b[16:20]) == "CMYK" && string(b[36:40]) == "acsp"
}
func safeOutput(input, dir string) string {
	name := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	name = strings.TrimSpace(name)
	if name == "" {
		name = "converted"
	}
	output := filepath.Join(dir, name+"_CMYK_FOGRA39.pdf")
	for i := 2; i < 10000; i++ {
		if _, err := os.Stat(output); os.IsNotExist(err) {
			return output
		}
		output = filepath.Join(dir, fmt.Sprintf("%s_CMYK_FOGRA39_%d.pdf", name, i))
	}
	return ""
}

// convertWithBundledGhostscript is used by both the GUI and the release smoke test.
func convertWithBundledGhostscript(gs, profile, libPath, input, dest string) error {
	args := []string{"-dBATCH", "-dNOPAUSE", "-dSAFER", "-sDEVICE=pdfwrite", "-dCompatibilityLevel=1.6", "-dProcessColorModel=/DeviceCMYK", "-sColorConversionStrategy=CMYK", "-sDefaultCMYKProfile=" + profile, "-dEmbedAllFonts=true", "-dSubsetFonts=true", "-sOutputFile=" + dest, input}
	cmd := exec.Command(gs, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = append(os.Environ(), "GS_LIB="+libPath)
	combined, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(dest)
		if len(combined) > 800 {
			combined = combined[len(combined)-800:]
		}
		return fmt.Errorf("Ghostscript: %w: %s", err, string(combined))
	}
	return nil
}
func bundleSmokeTest(input, output string) error {
	report := output + ".report.txt"
	write := func(msg string) { _ = os.WriteFile(report, []byte(msg+"\n"), 0644) }
	gs, profile, libPath, err := ensureBundledRuntime()
	if err != nil {
		write("ERROR extracting runtime: " + err.Error())
		return err
	}
	err = convertWithBundledGhostscript(gs, profile, libPath, input, output)
	if err != nil {
		write("ERROR converting: " + err.Error())
		return err
	}
	result := checkPDF(output)
	if result.Status != "CMYK" {
		err = fmt.Errorf("converted PDF classified as %s (%s)", result.Status, result.Failure)
		write("ERROR: " + err.Error())
		return err
	}
	if result.Pages < 1 {
		err = fmt.Errorf("no pages in result")
		write("ERROR: " + err.Error())
		return err
	}
	write("PASS: bundled Ghostscript converted test PDF to CMYK; pages=" + strconv.Itoa(result.Pages))
	return nil
}
func startConvertFOGRA39() {
	if running || converting {
		return
	}
	var chosen []pdfEntry
	for _, e := range entries {
		if e.Status == "RGB" || e.Status == "CMYK + RGB" {
			chosen = append(chosen, e)
		}
	}
	if len(chosen) == 0 {
		alert("Нет RGB или смешанных файлов для конвертации.")
		return
	}
	folder := chooseFolder(mainH)
	if folder == "" {
		return
	}
	convFolder = folder
	convTotal = len(chosen)
	convDone = 0
	convOK = 0
	convErrors = nil
	converting = true
	repaint()
	go func() {
		gs, profile, libPath, err := ensureBundledRuntime()
		if err != nil {
			convMutex.Lock()
			convErrors = append(convErrors, "Встроенный Ghostscript: "+err.Error())
			convMutex.Unlock()
			pPostMessage.Call(mainH, WM_APP_CONV_DONE, 0, 0)
			return
		}
		for _, e := range chosen {
			dest := safeOutput(e.Path, folder)
			var err error
			if dest == "" {
				err = fmt.Errorf("не удалось создать имя файла")
			}
			if err == nil {
				err = convertWithBundledGhostscript(gs, profile, libPath, e.Path, dest)
			}
			if err == nil {
				result := checkPDF(dest)
				if result.Status != "CMYK" {
					_ = os.Remove(dest)
					err = fmt.Errorf("не прошёл контроль цветов (%s)", result.Status)
				}
			}
			convMutex.Lock()
			if err != nil {
				convErrors = append(convErrors, e.Name+": "+err.Error())
			} else {
				convOK++
			}
			convMutex.Unlock()
			pPostMessage.Call(mainH, WM_APP_CONV_TICK, 0, 0)
		}
		pPostMessage.Call(mainH, WM_APP_CONV_DONE, 0, 0)
	}()
}

func onClick(x, y int32) {
	layoutButtons()
	for _, b := range buttons {
		if !inside(b.box, x, y) {
			continue
		}
		switch b.id {
		case 1:
			if running || converting {
				return
			}
			files, err := openPDFDialog()
			if err != nil {
				alert(err.Error() + "\nМожно также перетащить PDF в окно.")
			}
			for _, p := range files {
				enqueue(p)
			}
			launch()
		case 2:
			if running || converting {
				return
			}
			if dir := chooseFolder(mainH); dir != "" {
				enqueue(dir)
				launch()
			}
		case 3:
			if running || converting {
				return
			}
			previous := entries
			if len(previous) == 0 {
				alert("Сначала добавьте PDF-файлы.")
				return
			}
			entries = nil
			selected = -1
			offset = 0
			for _, e := range previous {
				pending = append(pending, e.Path)
			}
			launch()
		case 6:
			startConvertFOGRA39()
		case 4:
			showSaveWindow()
		case 5:
			if running || converting {
				alert("Подождите окончания обработки.")
				return
			}
			entries = nil
			pending = nil
			results = nil
			seen = map[string]bool{}
			done, total = 0, 0
			selected, offset = -1, 0
			repaint()
		}
		return
	}
	// select row without opening external files
	tableTop := int32(380)
	rowStart := tableTop + 44
	bottom := height - 58
	if y >= rowStart && y < bottom && x >= 28 && x < width-28 {
		idx := offset + int((y-rowStart)/38)
		if idx >= 0 && idx < len(entries) {
			selected = idx
			repaint()
		}
	}
	if y >= 35 && y < 70 && x >= width-174 && x <= width-27 {
		alert("PDF Color Checker 5.4\nПроверка цветовых пространств PDF.\n\nRGB, смешанные и неопределённые макеты можно выгрузить в TXT.\n\nПроверка справочная и не заменяет профессиональный Preflight.")
	}
}
func winProc(hwnd uintptr, m uint32, wp, lp uintptr) uintptr {
	switch m {
	case WM_GETMINMAXINFO:
		mm := (*minmaxinfo)(unsafe.Pointer(lp))
		mm.MinTrackSize = point{1060, 680}
		return 0
	case WM_CREATE:
		mainH = hwnd
		fontRegular = loadFont(19, 500)
		fontSmall = loadFont(17, 500)
		fontBold = loadFont(18, 700)
		fontTitle = loadFont(36, 800)
		fontSubtitle = loadFont(21, 500)
		fontTiny = loadFont(15, 600)
		pDragAccept.Call(hwnd, 1)
		return 0
	case WM_SIZE:
		width = int32(uint16(lp))
		height = int32(uint16(lp >> 16))
		repaint()
		return 0
	case 0x14:
		return 1 // WM_ERASEBKGND: double-buffer handles background
	case WM_PAINT:
		paintMain(hwnd)
		return 0
	case WM_LBUTTONUP:
		x, y := int32(int16(uint16(lp))), int32(int16(uint16(lp>>16)))
		onClick(x, y)
		return 0
	case WM_MOUSEMOVE:
		x, y := int32(int16(uint16(lp))), int32(int16(uint16(lp>>16)))
		old := hover
		hover = -1
		for _, b := range buttons {
			if inside(b.box, x, y) {
				hover = b.id
				break
			}
		}
		if hover != old {
			repaint()
		}
		return 0
	case WM_MOUSEWHEEL:
		delta := int16(uint16(wp >> 16))
		maxOff := maxInt(0, len(entries)-maxInt(1, int((height-58-380-45)/38)))
		if delta > 0 {
			offset -= 3
		} else if delta < 0 {
			offset += 3
		}
		if offset < 0 {
			offset = 0
		}
		if offset > maxOff {
			offset = maxOff
		}
		repaint()
		return 0
	case WM_DROPFILES:
		count, _, _ := pDragQuery.Call(wp, 0xffffffff, 0, 0)
		for i := uintptr(0); i < count; i++ {
			n, _, _ := pDragQuery.Call(wp, i, 0, 0)
			b := make([]uint16, n+1)
			pDragQuery.Call(wp, i, uintptr(unsafe.Pointer(&b[0])), n+1)
			enqueue(syscall.UTF16ToString(b))
		}
		pDragFinish.Call(wp)
		launch()
		return 0
	case WM_APP_RESULT:
		drain()
		return 0
	case WM_APP_CONV_TICK:
		convMutex.Lock()
		convDone++
		convMutex.Unlock()
		repaint()
		return 0
	case WM_APP_CONV_DONE:
		converting = false
		repaint()
		convMutex.Lock()
		ok := convOK
		errs := append([]string(nil), convErrors...)
		folder := convFolder
		convMutex.Unlock()
		notice := fmt.Sprintf("Готово. Преобразовано PDF: %d из %d.\nПапка: %s", ok, convTotal, folder)
		if len(errs) > 0 {
			notice += fmt.Sprintf("\nОшибок: %d\n", len(errs))
			for i, e := range errs {
				if i >= 3 {
					break
				}
				notice += "\n" + e
			}
		}
		alert(notice)
		return 0
	case WM_APP_DONE:
		drain()
		running = false
		repaint()
		if len(pending) > 0 {
			launch()
		}
		return 0
	case WM_DESTROY:
		if saveH != 0 {
			pDestroy.Call(saveH)
		}
		pPostQuit.Call(0)
		return 0
	}
	r, _, _ := pDefWindow.Call(hwnd, uintptr(m), wp, lp)
	return r
}
func main() {
	if len(os.Args) == 4 && os.Args[1] == "--bundle-smoke-test" {
		if err := bundleSmokeTest(os.Args[2], os.Args[3]); err != nil {
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--check" {
		for _, p := range os.Args[2:] {
			e := checkPDF(p)
			fmt.Printf("%s: %s, страницы=%d; %s\n", e.Name, e.Status, e.Pages, e.Failure)
		}
		return
	}
	runtime.LockOSThread()
	hr, _, _ := pCoInitialize.Call(0, 2)
	if hresult(hr) {
		defer pCoUninitialize.Call()
	}
	instance, _, _ := pGetModuleHandle.Call(0)
	iconH, _, _ = pLoadIcon.Call(instance, 1)
	saveClass := w("PDFCheckerSave5")
	saveWnd := wc{WndProc: syscall.NewCallback(saveProc), ClassName: uintptr(unsafe.Pointer(saveClass)), Icon: iconH, Instance: instance, Background: 6}
	pRegisterClass.Call(uintptr(unsafe.Pointer(&saveWnd)))
	classname := w("PDFColorChecker54")
	cursor, _, _ := pLoadCursor.Call(0, 32512)
	windowClass := wc{WndProc: syscall.NewCallback(winProc), ClassName: uintptr(unsafe.Pointer(classname)), Icon: iconH, Cursor: cursor, Instance: instance, Background: 6}
	pRegisterClass.Call(uintptr(unsafe.Pointer(&windowClass)))
	h, _, _ := pCreateWindow.Call(0, uintptr(unsafe.Pointer(classname)), ptr("PDF Color Checker 5.4"), WS_OVERLAPPEDWINDOW|WS_VISIBLE, 120, 80, 1180, 790, 0, 0, instance, 0)
	if h == 0 {
		fmt.Fprintln(os.Stderr, "Ошибка создания окна Windows")
		return
	}
	pShowWindow.Call(h, 1)
	pUpdateWindow.Call(h)
	var m msg
	for {
		n, _, _ := pGetMsg.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(n) <= 0 {
			break
		}
		pTranslate.Call(uintptr(unsafe.Pointer(&m)))
		pDispatch.Call(uintptr(unsafe.Pointer(&m)))
	}
}

var (
	objectHeaderRe = regexp.MustCompile(`(\d+)\s+\d+\s+obj\b`)
	iccRe          = regexp.MustCompile(`/N\s+(\d+)`)
	iccRefRe       = regexp.MustCompile(`/ICCBased\s+(\d+)\s+\d+\s+R`)
	rgbOpRe        = regexp.MustCompile(`(?:^|\s)(?:rg|RG)(?:\s|$)`)
	cmykOpRe       = regexp.MustCompile(`(?:^|\s)(?:k|K)(?:\s|$)`)
	pageRe         = regexp.MustCompile(`/Type\s*/Page\b`)
)

func checkPDF(path string) pdfEntry {
	e := pdfEntry{Name: filepath.Base(path), Path: path, Status: "Не определено"}
	info, err := os.Stat(path)
	if err != nil {
		e.Failure = "не удалось прочитать файл"
		return e
	}
	e.Size = info.Size()
	if e.Size > 512<<20 {
		e.Failure = "PDF больше 512 МБ, нужна ручная проверка"
		return e
	}
	b, err := os.ReadFile(path)
	if err != nil {
		e.Failure = "не удалось прочитать PDF"
		return e
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		e.Failure = "неверный формат PDF"
		return e
	}
	// Page markers live in object dictionaries. We don't guess if not recoverable.
	e.Pages = len(pageRe.FindAll(b, -1))
	objects := make(map[string][]byte)
	// Parse object boundaries without running a backtracking regexp across image streams.
	for pos := 0; pos < len(b); {
		loc := objectHeaderRe.FindSubmatchIndex(b[pos:])
		if loc == nil {
			break
		}
		objStart := pos + loc[1]
		id := string(b[pos+loc[2] : pos+loc[3]])
		objEndOffset := bytes.Index(b[objStart:], []byte("endobj"))
		if objEndOffset < 0 {
			break
		}
		objEnd := objStart + objEndOffset
		objects[id] = b[objStart:objEnd]
		pos = objEnd + 6
	}
	if len(objects) == 0 {
		e.Failure = "структура PDF не распознана"
		return e
	}
	var texts [][]byte
	for _, o := range objects {
		head := o
		if k := bytes.Index(o, []byte("stream")); k >= 0 {
			head = o[:k]
		}
		texts = append(texts, head)
		start := bytes.Index(o, []byte("stream"))
		if start < 0 {
			continue
		}
		start += 6
		if start < len(o) && o[start] == '\r' {
			start++
		}
		if start < len(o) && o[start] == '\n' {
			start++
		}
		end := bytes.Index(o[start:], []byte("endstream"))
		if end < 0 {
			continue
		}
		raw := o[start : start+end]
		if bytes.Contains(head, []byte("/ASCII85Decode")) {
			raw = bytes.TrimSpace(raw)
			raw = bytes.TrimPrefix(raw, []byte("<~"))
			raw = bytes.TrimSuffix(raw, []byte("~>"))
			decoded, derr := io.ReadAll(io.LimitReader(ascii85.NewDecoder(bytes.NewReader(raw)), 32<<20))
			if derr == nil {
				raw = decoded
			} else {
				raw = nil
			}
		}
		if raw != nil && bytes.Contains(head, []byte("/FlateDecode")) {
			z, zerr := zlib.NewReader(bytes.NewReader(raw))
			if zerr == nil {
				dec, derr := io.ReadAll(io.LimitReader(z, 32<<20))
				z.Close()
				if derr == nil {
					raw = dec
				} else {
					raw = nil
				}
			} else {
				raw = nil
			}
		} else if bytes.Contains(head, []byte("/Filter")) && !bytes.Contains(head, []byte("/ASCII85Decode")) {
			raw = nil
		}
		if raw != nil && len(raw) < 32<<20 {
			texts = append(texts, raw)
		}
	}
	all := bytes.Join(texts, []byte("\n"))
	e.RGB = bytes.Contains(all, []byte("/DeviceRGB")) || bytes.Contains(all, []byte("/CalRGB"))
	e.CMYK = bytes.Contains(all, []byte("/DeviceCMYK"))
	for _, m := range iccRefRe.FindAllSubmatch(all, -1) {
		if o, ok := objects[string(m[1])]; ok {
			if channels := iccRe.FindSubmatch(o); len(channels) > 1 {
				count, _ := strconv.Atoi(string(channels[1]))
				if count == 3 {
					e.RGB = true
				}
				if count == 4 {
					e.CMYK = true
				}
			}
		}
	}
	if rgbOpRe.Match(all) {
		e.RGB = true
	}
	if cmykOpRe.Match(all) {
		e.CMYK = true
	}
	switch {
	case e.RGB && e.CMYK:
		e.Status = "CMYK + RGB"
	case e.RGB:
		e.Status = "RGB"
	case e.CMYK:
		e.Status = "CMYK"
	default:
		e.Failure = "цветовая модель не определена"
	}
	return e
}