package woxview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	component "wox/ui/launcher/component"
)

// Size is a literal logical length or a semantic dimension token. Values remain
// unresolved in the document so the same tree can follow Interface size changes.
type Size struct {
	Value float32
	Token SizeToken
}

// Offset accepts signed logical lengths and the same dimension tokens as Size.
type Offset Size

func (o Offset) resolve(theme component.ControlTheme) float32 { return Size(o).resolve(theme) }
func (o Offset) validate() error                              { return Size(o).validateRange(-16384) }
func (o Offset) MarshalJSON() ([]byte, error)                 { return Size(o).marshalJSON(-16384) }

// UnmarshalJSON preserves signed offsets without allowing negative layout sizes.
func (o *Offset) UnmarshalJSON(data []byte) error {
	next, err := decodeSize(data, -16384)
	if err == nil {
		*o = Offset(next)
	}
	return err
}

type SizeToken string

const (
	SizeRadiusSmall  SizeToken = "$radiusSm"
	SizeRadiusMedium SizeToken = "$radiusMd"
	SizeSpace1       SizeToken = "$space1"
	SizeSpace2       SizeToken = "$space2"
	SizeSpace3       SizeToken = "$space3"
	SizeSpace4       SizeToken = "$space4"
	SizeFontSmall    SizeToken = "$fontSm"
	SizeFontMedium   SizeToken = "$fontMd"
	SizeFontLarge    SizeToken = "$fontLg"
	SizeFontTitle    SizeToken = "$fontTitle"
)

func (SizeToken) enumValues() []string {
	return []string{string(SizeRadiusSmall), string(SizeRadiusMedium), string(SizeSpace1), string(SizeSpace2), string(SizeSpace3), string(SizeSpace4), string(SizeFontSmall), string(SizeFontMedium), string(SizeFontLarge), string(SizeFontTitle)}
}

// UnmarshalJSON accepts only finite lengths and known tokens, not CSS expressions.
func (s *Size) UnmarshalJSON(data []byte) error {
	next, err := decodeSize(data, 0)
	if err == nil {
		*s = next
	}
	return err
}

// decodeSize shares token parsing while keeping each domain's numeric lower bound.
func decodeSize(data []byte, minimum float32) (Size, error) {
	data = bytes.TrimSpace(data)
	var next Size
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &next.Token); err != nil {
			return Size{}, err
		}
		if next.Token == "" {
			return Size{}, errors.New("size token cannot be empty")
		}
	} else if err := json.Unmarshal(data, &next.Value); err != nil {
		return Size{}, errors.New("size must be a number or a dimension token")
	}
	if err := next.validateRange(minimum); err != nil {
		return Size{}, err
	}
	return next, nil
}

// MarshalJSON preserves token identity rather than freezing it to a resolved length.
func (s Size) MarshalJSON() ([]byte, error) {
	return s.marshalJSON(0)
}

// marshalJSON validates the declared domain while retaining the original token.
func (s Size) marshalJSON(minimum float32) ([]byte, error) {
	if err := s.validateRange(minimum); err != nil {
		return nil, err
	}
	if s.Token != "" {
		return json.Marshal(s.Token)
	}
	return json.Marshal(s.Value)
}

// base returns normal-density token metrics; typography owners scale these once.
func (s Size) base() float32 {
	switch s.Token {
	case SizeRadiusSmall, SizeSpace1:
		return 4
	case SizeRadiusMedium, SizeSpace2:
		return 8
	case SizeSpace3:
		return 12
	case SizeSpace4:
		return 16
	case SizeFontSmall:
		return component.SettingsSecondaryFontSize
	case SizeFontMedium:
		return component.SettingsControlFontSize
	case SizeFontLarge:
		return 16
	case SizeFontTitle:
		return component.SettingsPageTitleFontSize
	default:
		return s.Value
	}
}

// resolve scales tokens at the logical layout boundary. Numeric lengths retain
// their previous semantics; DPI conversion remains the renderer's responsibility.
func (s Size) resolve(theme component.ControlTheme) float32 {
	if s.Token != "" {
		return theme.Scaled(s.base())
	}
	return s.Value
}

// validate also guards programmatic trees, which do not pass through JSON decoding.
func (s Size) validate() error {
	return s.validateRange(0)
}

// validateRange also rejects mixed value/token states in programmatic documents.
func (s Size) validateRange(minimum float32) error {
	if s.Token != "" {
		if s.Value != 0 {
			return errors.New("size cannot contain both a value and a token")
		}
		for _, token := range (SizeToken("")).enumValues() {
			if string(s.Token) == token {
				return nil
			}
		}
		return fmt.Errorf("unknown size token %q", s.Token)
	}
	if s.Value < minimum || s.Value > 16384 || math.IsNaN(float64(s.Value)) {
		return fmt.Errorf("size must be between %g and 16384", minimum)
	}
	return nil
}
