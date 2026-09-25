package puush

type AccountType int8

const (
	AccountTypeRegular AccountType = iota
	AccountTypePro
	AccountTypeUnlimited
)

func (at AccountType) String() string {
	switch at {
	case AccountTypeRegular:
		return "Free"
	case AccountTypePro:
		return "Pro"
	case AccountTypeUnlimited:
		return "Unlimited"
	default:
		return "Unknown"
	}
}

const (
	UploadLimitRegular = 200 * 1024 * 1024       // 200 MB
	UploadLimitPro     = 15 * 1000 * 1024 * 1024 // 15 GB
)
