package notifications

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/sundeiii/yeet-client/assets"
)

// Sounds for finished uploads. "puush" is the classic one; the others are
// made here, so they need no files.
const (
	SoundPuush  = "puush"
	SoundPop    = "pop"
	SoundChime  = "chime"
	SoundSystem = "system" // the system's own notification sound
	SoundNone   = "none"
)

// SoundChoices are the sounds in the order the settings list them.
var SoundChoices = []string{SoundPuush, SoundPop, SoundChime, SoundSystem, SoundNone}

// SoundData returns a sound as WAV data, or nil for system and none.
func SoundData(name string) []byte {
	switch name {
	case SoundPuush:
		return assets.SuccessSoundData
	case SoundPop:
		return synthesize([]tone{{start: 0, length: 0.09, from: 950, to: 320, volume: 0.8}})
	case SoundChime:
		return synthesize([]tone{
			{start: 0, length: 0.45, from: 1318.5, to: 1318.5, volume: 0.45},
			{start: 0.11, length: 0.6, from: 1760, to: 1760, volume: 0.45},
		})
	}
	return nil
}

type tone struct {
	start, length float64 // seconds
	from, to      float64 // frequency sweep, Hz
	volume        float64
}

const sampleRate = 44100

// synthesize mixes tones with a quick fade-in and a smooth decay into a
// 16-bit mono WAV file.
func synthesize(tones []tone) []byte {
	end := 0.0
	for _, t := range tones {
		end = math.Max(end, t.start+t.length)
	}
	samples := make([]float64, int(end*sampleRate)+1)
	for _, t := range tones {
		first := int(t.start * sampleRate)
		count := int(t.length * sampleRate)
		phase := 0.0
		for i := 0; i < count && first+i < len(samples); i++ {
			progress := float64(i) / float64(count)
			frequency := t.from + (t.to-t.from)*progress
			phase += 2 * math.Pi * frequency / sampleRate
			attack := math.Min(1, float64(i)/(0.004*sampleRate))
			decay := math.Exp(-5 * progress)
			samples[first+i] += math.Sin(phase) * t.volume * attack * decay
		}
	}

	var buffer bytes.Buffer
	dataSize := uint32(len(samples) * 2)
	buffer.WriteString("RIFF")
	binary.Write(&buffer, binary.LittleEndian, 36+dataSize)
	buffer.WriteString("WAVEfmt ")
	binary.Write(&buffer, binary.LittleEndian, uint32(16))
	binary.Write(&buffer, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&buffer, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(2))
	binary.Write(&buffer, binary.LittleEndian, uint16(16))
	buffer.WriteString("data")
	binary.Write(&buffer, binary.LittleEndian, dataSize)
	for _, sample := range samples {
		value := math.Max(-1, math.Min(1, sample))
		binary.Write(&buffer, binary.LittleEndian, int16(value*32000))
	}
	return buffer.Bytes()
}

var (
	soundFiles   = map[string]string{}
	soundFilesMu sync.Mutex
)

// soundFile writes a sound to a temporary file once and returns its path,
// since the players want a file.
func soundFile(name string) string {
	soundFilesMu.Lock()
	defer soundFilesMu.Unlock()
	if path, ok := soundFiles[name]; ok {
		return path
	}
	data := SoundData(name)
	if data == nil {
		return ""
	}
	path := filepath.Join(os.TempDir(), "yeet-sound-"+name+".wav")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return ""
	}
	soundFiles[name] = path
	return path
}

// PlaySound plays one of the sounds without waiting for it to finish.
func PlaySound(name string) {
	if path := soundFile(name); path != "" {
		go playFile(path)
	}
}
