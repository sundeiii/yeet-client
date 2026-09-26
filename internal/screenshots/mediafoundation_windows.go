//go:build windows

package screenshots

import (
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MP4 videos are made with Windows' own H.264 encoder through Media
// Foundation, which every Windows 10 and 11 has (except "N" editions
// without the Media Feature Pack). Its COM interfaces are called through
// their method tables.

var (
	modMfplat      = windows.NewLazySystemDLL("mfplat.dll")
	modMfreadwrite = windows.NewLazySystemDLL("mfreadwrite.dll")
	modOle32       = windows.NewLazySystemDLL("ole32.dll")

	procMFStartup                 = modMfplat.NewProc("MFStartup")
	procMFShutdown                = modMfplat.NewProc("MFShutdown")
	procMFCreateMediaType         = modMfplat.NewProc("MFCreateMediaType")
	procMFCreateSample            = modMfplat.NewProc("MFCreateSample")
	procMFCreateMemoryBuffer      = modMfplat.NewProc("MFCreateMemoryBuffer")
	procMFCreateAttributes        = modMfplat.NewProc("MFCreateAttributes")
	procMFCreateSinkWriterFromURL = modMfreadwrite.NewProc("MFCreateSinkWriterFromURL")
	procCoInitializeEx            = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize            = modOle32.NewProc("CoUninitialize")
)

const (
	mfVersion           = 0x00020070
	coinitMultithreaded = 0x0
	rpcEChangedMode     = 0x80010106
	mfVideoProgressive  = 2
	h264ProfileHigh     = 100
)

var (
	mfMTMajorType          = guid("48eba18e-f8c9-4687-bf11-0a74c9f96a8f")
	mfMTSubtype            = guid("f7e34c9a-42e8-4714-b74b-cb29d72c35e5")
	mfMTAvgBitrate         = guid("20332624-fb0d-4d9e-bd0d-cbf6786c102e")
	mfMTInterlaceMode      = guid("e2724bb8-e676-4806-b4b2-a8d6efb44ccd")
	mfMTFrameSize          = guid("1652c33d-d6b2-4012-b834-72030849a37d")
	mfMTFrameRate          = guid("c459a2e8-3d2c-4e44-b132-fee5156c7bb0")
	mfMTPixelAspectRatio   = guid("c6376a1e-8d0a-4027-be45-6d9a0ad39bb6")
	mfMTDefaultStride      = guid("644b4e48-1e02-4516-b0eb-c01ca9d49ac6")
	mfMTMpeg2Profile       = guid("ad76a80b-2d5c-4e0b-b375-64e520137036")
	mfMediaTypeVideo       = guid("73646976-0000-0010-8000-00aa00389b71")
	mfVideoFormatH264      = guid("34363248-0000-0010-8000-00aa00389b71")
	mfVideoFormatRGB32     = guid("00000016-0000-0010-8000-00aa00389b71")
	mfReadwriteEnableHwMFT = guid("a634a91c-822b-41b9-a494-4de4643612b0")
)

func guid(text string) *windows.GUID {
	g, err := windows.GUIDFromString("{" + text + "}")
	if err != nil {
		panic(err)
	}
	return &g
}

// Method numbers in the COM interfaces' tables
const (
	methodRelease = 2

	// IMFAttributes (and IMFMediaType, IMFSample, which extend it)
	methodSetUINT32 = 21
	methodSetUINT64 = 22
	methodSetGUID   = 24

	// IMFSample
	methodSetSampleTime     = 36
	methodSetSampleDuration = 38
	methodAddBuffer         = 42

	// IMFMediaBuffer
	methodLock             = 3
	methodUnlock           = 4
	methodSetCurrentLength = 6

	// IMFSinkWriter
	methodAddStream         = 3
	methodSetInputMediaType = 4
	methodBeginWriting      = 5
	methodWriteSample       = 6
	methodFinalize          = 11
)

type comObject unsafe.Pointer

func comCall(object comObject, method int, args ...uintptr) error {
	table := *(*unsafe.Pointer)(object)
	function := *(*uintptr)(unsafe.Add(table, method*int(unsafe.Sizeof(uintptr(0)))))
	r1, _, _ := syscall.SyscallN(function, append([]uintptr{uintptr(object)}, args...)...)
	return hresult(r1)
}

func comRelease(object comObject) {
	if object != nil {
		comCall(object, methodRelease)
	}
}

func hresult(r1 uintptr) error {
	if int32(r1) < 0 {
		return fmt.Errorf("HRESULT 0x%08X", uint32(r1))
	}
	return nil
}

func callProc(proc *windows.LazyProc, args ...uintptr) error {
	if err := proc.Find(); err != nil {
		return err
	}
	r1, _, _ := proc.Call(args...)
	if err := hresult(r1); err != nil {
		return fmt.Errorf("%s: %w", proc.Name, err)
	}
	return nil
}

// mp4Writer encodes frames of BGRA pixels into an MP4 file.
type mp4Writer struct {
	writer         comObject
	stream         uint32
	width, height  int
	comInitialized bool
	mfStarted      bool
}

// newMP4Writer starts a video. Width and height must be even. It has to be
// used from one locked OS thread.
func newMP4Writer(path string, width, height, fps int) (*mp4Writer, error) {
	w := &mp4Writer{width: width, height: height}
	r1, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	switch {
	case uint32(r1) == rpcEChangedMode:
		// This thread already uses COM another way, which works too
	case hresult(r1) != nil:
		return nil, fmt.Errorf("CoInitializeEx: %w", hresult(r1))
	default:
		w.comInitialized = true
	}
	if err := callProc(procMFStartup, mfVersion, 0); err != nil {
		w.release()
		return nil, fmt.Errorf("media foundation isn't available: %w", err)
	}
	w.mfStarted = true

	// A graphics card's encoder if there is one, Windows' own otherwise
	var err error
	for _, hardware := range []bool{true, false} {
		if err = w.open(path, fps, hardware); err == nil {
			return w, nil
		}
		comRelease(w.writer)
		w.writer = nil
	}
	w.release()
	return nil, err
}

func (w *mp4Writer) open(path string, fps int, hardware bool) error {
	var attributes comObject
	if err := callProc(procMFCreateAttributes, uintptr(unsafe.Pointer(&attributes)), 1); err != nil {
		return err
	}
	defer comRelease(attributes)
	enable := uintptr(0)
	if hardware {
		enable = 1
	}
	if err := comCall(attributes, methodSetUINT32, uintptr(unsafe.Pointer(mfReadwriteEnableHwMFT)), enable); err != nil {
		return err
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := callProc(procMFCreateSinkWriterFromURL, uintptr(unsafe.Pointer(pathPtr)), 0, uintptr(attributes), uintptr(unsafe.Pointer(&w.writer))); err != nil {
		return err
	}

	// Screens have sharp text, which needs more bits than camera videos
	bitrate := min(max(w.width*w.height*fps/6, 1_000_000), 16_000_000)
	output, err := w.mediaType(mfVideoFormatH264, fps, func(t comObject) error {
		if err := comCall(t, methodSetUINT32, uintptr(unsafe.Pointer(mfMTAvgBitrate)), uintptr(bitrate)); err != nil {
			return err
		}
		return comCall(t, methodSetUINT32, uintptr(unsafe.Pointer(mfMTMpeg2Profile)), h264ProfileHigh)
	})
	if err != nil {
		return err
	}
	defer comRelease(output)
	if err := comCall(w.writer, methodAddStream, uintptr(output), uintptr(unsafe.Pointer(&w.stream))); err != nil {
		return fmt.Errorf("add stream: %w", err)
	}

	input, err := w.mediaType(mfVideoFormatRGB32, fps, func(t comObject) error {
		// Rows go from the top down, like the captured pixels
		return comCall(t, methodSetUINT32, uintptr(unsafe.Pointer(mfMTDefaultStride)), uintptr(uint32(w.width*4)))
	})
	if err != nil {
		return err
	}
	defer comRelease(input)
	if err := comCall(w.writer, methodSetInputMediaType, uintptr(w.stream), uintptr(input), 0); err != nil {
		return fmt.Errorf("set input: %w", err)
	}
	if err := comCall(w.writer, methodBeginWriting); err != nil {
		return fmt.Errorf("begin writing: %w", err)
	}
	return nil
}

func (w *mp4Writer) mediaType(subtype *windows.GUID, fps int, extra func(comObject) error) (comObject, error) {
	var t comObject
	if err := callProc(procMFCreateMediaType, uintptr(unsafe.Pointer(&t))); err != nil {
		return nil, err
	}
	steps := []error{
		comCall(t, methodSetGUID, uintptr(unsafe.Pointer(mfMTMajorType)), uintptr(unsafe.Pointer(mfMediaTypeVideo))),
		comCall(t, methodSetGUID, uintptr(unsafe.Pointer(mfMTSubtype)), uintptr(unsafe.Pointer(subtype))),
		comCall(t, methodSetUINT32, uintptr(unsafe.Pointer(mfMTInterlaceMode)), mfVideoProgressive),
		comCall(t, methodSetUINT64, uintptr(unsafe.Pointer(mfMTFrameSize)), uintptr(uint64(w.width)<<32|uint64(w.height))),
		comCall(t, methodSetUINT64, uintptr(unsafe.Pointer(mfMTFrameRate)), uintptr(uint64(fps)<<32|1)),
		comCall(t, methodSetUINT64, uintptr(unsafe.Pointer(mfMTPixelAspectRatio)), uintptr(uint64(1)<<32|1)),
		extra(t),
	}
	if err := errors.Join(steps...); err != nil {
		comRelease(t)
		return nil, fmt.Errorf("media type: %w", err)
	}
	return t, nil
}

// writeFrame adds a frame of BGRA pixels, top row first, shown from the
// given time since the start.
func (w *mp4Writer) writeFrame(bgra []byte, at time.Duration) error {
	size := w.width * w.height * 4
	if len(bgra) < size {
		return errors.New("frame too small")
	}
	var buffer comObject
	if err := callProc(procMFCreateMemoryBuffer, uintptr(size), uintptr(unsafe.Pointer(&buffer))); err != nil {
		return err
	}
	defer comRelease(buffer)
	var data *byte
	if err := comCall(buffer, methodLock, uintptr(unsafe.Pointer(&data)), 0, 0); err != nil {
		return err
	}
	copy(unsafe.Slice(data, size), bgra[:size])
	comCall(buffer, methodUnlock)
	if err := comCall(buffer, methodSetCurrentLength, uintptr(size)); err != nil {
		return err
	}

	var sample comObject
	if err := callProc(procMFCreateSample, uintptr(unsafe.Pointer(&sample))); err != nil {
		return err
	}
	defer comRelease(sample)
	if err := comCall(sample, methodAddBuffer, uintptr(buffer)); err != nil {
		return err
	}
	// Media Foundation counts in 100 nanosecond steps
	start := at.Nanoseconds() / 100
	duration := max(int64(time.Second/videoFps)/100, 1)
	comCall(sample, methodSetSampleTime, uintptr(start))
	comCall(sample, methodSetSampleDuration, uintptr(duration))
	if err := comCall(w.writer, methodWriteSample, uintptr(w.stream), uintptr(sample)); err != nil {
		return fmt.Errorf("write sample: %w", err)
	}
	return nil
}

// finish completes the file.
func (w *mp4Writer) finish() error {
	err := comCall(w.writer, methodFinalize)
	w.release()
	if err != nil {
		return fmt.Errorf("finalize: %w", err)
	}
	return nil
}

func (w *mp4Writer) release() {
	comRelease(w.writer)
	w.writer = nil
	if w.mfStarted {
		procMFShutdown.Call()
		w.mfStarted = false
	}
	if w.comInitialized {
		procCoUninitialize.Call()
		w.comInitialized = false
	}
}
