package game

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (c *Client) Subscribe(ctx context.Context) (<-chan BridgeEvent, <-chan error) {
	eventCh := make(chan BridgeEvent, 32)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		if err := c.streamEvents(ctx, eventCh); err != nil && ctx.Err() == nil {
			errCh <- err
		}
	}()

	return eventCh, errCh
}

func (c *Client) streamEvents(ctx context.Context, eventCh chan<- BridgeEvent) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/events/stream", nil)
	if err != nil {
		return fmt.Errorf("build event stream request: %w", err)
	}

	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("get /events/stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var envelope Envelope[json.RawMessage]
		if decodeErr := decodeEnvelope(resp, &envelope); decodeErr != nil {
			return fmt.Errorf("event stream unavailable: http %d", resp.StatusCode)
		}

		if envelope.Error != nil {
			return fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
		}

		return fmt.Errorf("event stream unavailable: http %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(dataLines) > 0 {
				payload := strings.Join(dataLines, "\n")
				var bridgeEvent BridgeEvent
				if err := json.Unmarshal([]byte(payload), &bridgeEvent); err == nil {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case eventCh <- bridgeEvent:
					}
				}

				dataLines = dataLines[:0]
			}

			continue
		}

		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("read event stream: %w", err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	return nil
}
