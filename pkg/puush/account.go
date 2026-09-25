package puush

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Credentials represents the authentication credentials for a puush account.
// It can either contain an API key or a combination of username/email & password for login.
type Credentials struct {
	Identifier *string
	Password   *string
	Key        *string
}

func (c *Credentials) HasApiKey() bool {
	return c.Identifier != nil && c.Key != nil
}

func (c *Credentials) HasLoginCredentials() bool {
	return c.Identifier != nil && c.Password != nil
}

func (c *Credentials) IsValid() bool {
	return c.HasApiKey() || c.HasLoginCredentials()
}

func (c *Credentials) Reset() {
	c.Identifier = nil
	c.Password = nil
	c.Key = nil
}

func (c *Credentials) toFormData() url.Values {
	params := url.Values{}
	if c.HasApiKey() {
		params.Add("e", *c.Identifier)
		params.Add("k", *c.Key)
	} else if c.HasLoginCredentials() {
		params.Add("e", *c.Identifier)
		params.Add("p", *c.Password)
	}
	return params
}

// Account represents a puush account with its credentials, type, disk usage, and pro subscription end date.
// It can be obtained by calling `Authenticate()` on the Client struct, assuming the credentials are provided.
type Account struct {
	Credentials     *Credentials
	Type            AccountType
	DiskUsage       int64
	SubscriptionEnd *time.Time
}

func (a *Account) DiskUsageHumanReadable() string {
	return formatBytes(a.DiskUsage)
}

func (a *Account) CanUpload() bool {
	limit := a.UploadLimit()
	return limit == -1 || a.DiskUsage < limit
}

func (a *Account) UploadLimit() int64 {
	switch a.Type {
	case AccountTypeRegular:
		return UploadLimitRegular
	case AccountTypePro:
		return UploadLimitPro
	default:
		return -1
	}
}

func (a *Account) Reset() {
	a.Credentials.Reset()
	a.Type = AccountTypeRegular
	a.DiskUsage = 0
	a.SubscriptionEnd = nil
}

func NewAccountFromCredentials(creds *Credentials) (*Account, error) {
	return &Account{
		Credentials:     creds,
		Type:            AccountTypeRegular,
		DiskUsage:       0,
		SubscriptionEnd: nil,
	}, nil
}

func NewAccountFromResponse(scanner *bufio.Scanner) (*Account, error) {
	authenticationResponse := strings.Split(scanner.Text(), ",")

	accountTypeInt, err := strconv.Atoi(authenticationResponse[0])
	if err != nil {
		return nil, errors.New("invalid account type in response")
	}
	accountType := AccountType(accountTypeInt)

	apiKey := authenticationResponse[1]
	subscriptionEndStr := authenticationResponse[2]

	var subscriptionEnd *time.Time

	if subscriptionEndStr != "" {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", subscriptionEndStr)
		if err != nil {
			return nil, errors.New("invalid subscription end date format in response")
		}
		subscriptionEnd = &parsedTime
	}

	diskUsage, err := strconv.ParseInt(authenticationResponse[3], 10, 64)
	if err != nil {
		return nil, errors.New("invalid disk usage value in response")
	}

	return &Account{
		Type:            accountType,
		DiskUsage:       diskUsage,
		SubscriptionEnd: subscriptionEnd,
		Credentials:     &Credentials{Key: &apiKey},
	}, nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.2f%s", float64(bytes)/float64(div), units[exp])
}
