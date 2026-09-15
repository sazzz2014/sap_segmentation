package model

import (
	"errors"
	"strings"
	"unicode/utf8"
)

type Segmentation struct {
	ID           int64  `db:"id" json:"-"`
	AddressSAPID string `db:"address_sap_id" json:"address_sap_id"`
	AdrSegment   string `db:"adr_segment" json:"adr_segment"`
	SegmentID    int64  `db:"segment_id" json:"segment_id"`
}

func (s Segmentation) Validate() error {
	if !utf8.ValidString(s.AddressSAPID) || strings.ContainsRune(s.AddressSAPID, 0) || strings.TrimSpace(s.AddressSAPID) == "" || utf8.RuneCountInString(s.AddressSAPID) > 255 {
		return errors.New("address_sap_id must contain 1..255 characters and no NUL")
	}
	if !utf8.ValidString(s.AdrSegment) || strings.ContainsRune(s.AdrSegment, 0) || utf8.RuneCountInString(s.AdrSegment) > 16 {
		return errors.New("adr_segment must contain at most 16 characters and no NUL")
	}
	return nil
}
