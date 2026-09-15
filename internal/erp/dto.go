package erp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"sap_segmentation/model"
)

type segmentationDTO struct {
	AddressSAPID *string `json:"address_sap_id"`
	AdrSegment   *string `json:"adr_segment"`
	SegmentID    *int64  `json:"segment_id"`
}

func decodePage(body []byte) ([]model.Segmentation, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		return nil, nil
	}
	var rows []segmentationDTO
	if body[0] == '{' {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, errors.New("invalid JSON envelope")
		}
		items, ok := envelope["items"]
		if !ok {
			return nil, errors.New("JSON object must contain items")
		}
		if err := json.Unmarshal(items, &rows); err != nil {
			return nil, errors.New("invalid JSON items")
		}
	} else if err := json.Unmarshal(body, &rows); err != nil {
		return nil, errors.New("invalid JSON array")
	}
	result := make([]model.Segmentation, 0, len(rows))
	for i, row := range rows {
		if row.AddressSAPID == nil || row.AdrSegment == nil || row.SegmentID == nil {
			return nil, fmt.Errorf("record %d: required field missing or null", i)
		}
		s := model.Segmentation{
			AddressSAPID: *row.AddressSAPID,
			AdrSegment:   *row.AdrSegment,
			SegmentID:    *row.SegmentID,
		}
		if err := s.Validate(); err != nil {
			return nil, fmt.Errorf("record %d: %w", i, err)
		}
		result = append(result, s)
	}
	return result, nil
}
