package shared

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type Address struct {
	Line1       string      `json:"line_1,omitempty"`
	Line2       string      `json:"line_2,omitempty"`
	PostalCode  string      `json:"postal_code,omitempty"`
	City        string      `json:"city,omitempty"`
	CountryCode CountryCode `json:"country_code"`
}

func NewAddress(line1, line2, postalCode, city string, countryCode CountryCode) (Address, error) {
	if line1 == "" {
		return Address{}, errors.New("address line 1 is required")
	}

	if postalCode == "" {
		return Address{}, errors.New("postal code is required")
	}

	if city == "" {
		return Address{}, errors.New("city is required")
	}

	if countryCode.IsZero() {
		return Address{}, errors.New("country code is required")
	}

	return Address{
		Line1:       line1,
		Line2:       line2,
		PostalCode:  postalCode,
		City:        city,
		CountryCode: countryCode,
	}, nil
}

func (a Address) IsZero() bool {
	return a == Address{}
}

func (a *Address) Scan(src any) error {
	var text []byte

	switch t := src.(type) {
	case string:
		text = []byte(t)
	case []byte:
		text = t
	default:
		return fmt.Errorf("invalid address type: %T", t)
	}

	err := json.Unmarshal(text, a)
	if err != nil {
		return fmt.Errorf("error unmarshalling %T from json: %w", a, err)
	}

	return nil
}

func (a Address) Value() (driver.Value, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("error marshalling %T to json: %w", a, err)
	}

	return string(data), nil
}
