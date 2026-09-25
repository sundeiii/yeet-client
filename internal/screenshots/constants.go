package screenshots

type Quality int

const (
	QualityBest Quality = iota
	QualityHigh
	QualityMedium
	QualityLow
)

func (quality Quality) Value() int {
	switch quality {
	case QualityBest:
		return 100
	case QualityHigh:
		return 85
	case QualityMedium:
		return 60
	case QualityLow:
		return 30
	default:
		return 85
	}
}

type FullscreenMode int

const (
	FullscreenModeMouse FullscreenMode = iota
	FullscreenModeAllScreens
	FullscreenModePrimary
)
