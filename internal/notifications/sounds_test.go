package notifications

import (
	"encoding/binary"
	"testing"
)

func TestSounds(t *testing.T) {
	for _, name := range []string{SoundPop, SoundChime} {
		data := SoundData(name)
		if len(data) < 1000 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
			t.Fatalf("%s is not a WAV file", name)
		}
		if size := binary.LittleEndian.Uint32(data[40:44]); int(size) != len(data)-44 {
			t.Errorf("%s: data size %d doesn't match %d bytes", name, size, len(data)-44)
		}
	}
	if SoundData(SoundNone) != nil || SoundData(SoundSystem) != nil {
		t.Error("no data for none and system")
	}
	if len(SoundData(SoundPuush)) == 0 {
		t.Error("the classic sound should be there")
	}
}
