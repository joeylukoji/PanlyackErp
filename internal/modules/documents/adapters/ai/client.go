package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"panlyackerp/internal/modules/documents/domain"
)

type Client struct {
	HTTP         *http.Client
	BaseURL, Key string
}

func (c Client) Extract(ctx context.Context, data []byte, mime string) (domain.Invoice, error) {
	invoice := domain.Invoice{}
	body, err := json.Marshal(map[string]string{"file_base64": base64.StdEncoding.EncodeToString(data), "mime_type": mime})
	if err != nil {
		return invoice, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/invoices/extract", bytes.NewReader(body))
	if err != nil {
		return invoice, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	response, err := c.HTTP.Do(req)
	if err != nil {
		return invoice, fmt.Errorf("extraction service unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return invoice, errors.New("extraction failed; check the AI service configuration and retry")
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&invoice); err != nil {
		return invoice, fmt.Errorf("invalid extraction response: %w", err)
	}
	if len(invoice.Lines) > 100 {
		return invoice, errors.New("extraction returned too many lines")
	}
	return invoice, nil
}
