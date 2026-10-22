package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coalaura/openingrouter"
)

type ChatStreamStarter func(context.Context, openingrouter.ChatCompletionRequest) (openingrouter.OpenrouterStream[openingrouter.ChatStreamChunk], error)

type ChatRetrySettings struct {
	MaxRetries   int   `json:"max_retries"`
	InitialDelay int64 `json:"initial_delay"`
	MaxDelay     int64 `json:"max_delay"`
}

type RetryingChatStream struct {
	ctx       context.Context
	request   openingrouter.ChatCompletionRequest
	settings  ChatRetrySettings
	start     ChatStreamStarter
	notify    func(RetryChunk) error
	stream    openingrouter.OpenrouterStream[openingrouter.ChatStreamChunk]
	retries   int
	hasOutput bool
}

func (r ChatRetrySettings) Validate() error {
	if r.MaxRetries < 0 || r.MaxRetries > 100 {
		return errors.New("retry.max_retries must be between 0 and 100")
	}

	if r.InitialDelay < 1 || r.InitialDelay > 3600 {
		return errors.New("retry.initial_delay must be between 1 and 3600 seconds")
	}

	if r.MaxDelay < r.InitialDelay || r.MaxDelay > 3600 {
		return errors.New("retry.max_delay must be between initial_delay and 3600 seconds")
	}

	return nil
}

func (r ChatRetrySettings) delay(retries int) time.Duration {
	delay := time.Duration(r.InitialDelay) * time.Second
	maximum := time.Duration(r.MaxDelay) * time.Second

	for range retries {
		if delay >= maximum {
			break
		}

		delay = min(delay*2, maximum)
	}

	return delay
}

func (s *RetryingChatStream) Recv() (openingrouter.ChatStreamChunk, error) {
	for {
		err := s.ctx.Err()
		if err != nil {
			return openingrouter.ChatStreamChunk{}, err
		}

		chunk, err := s.stream.Recv()
		if err == nil {
			s.hasOutput = s.hasOutput || completionChunkHasOutput(&chunk)

			return chunk, nil
		}

		err = s.retry(err)
		if err != nil {
			return openingrouter.ChatStreamChunk{}, err
		}

		err = s.open()
		if err != nil {
			return openingrouter.ChatStreamChunk{}, err
		}
	}
}

func (s *RetryingChatStream) Close() {
	if s.stream != nil {
		s.stream.Close()
		s.stream = nil
	}
}

func (s *RetryingChatStream) open() error {
	for {
		err := s.ctx.Err()
		if err != nil {
			return err
		}

		s.stream, err = s.start(s.ctx, s.request)
		if err == nil {
			return nil
		}

		err = s.retry(err)
		if err != nil {
			return err
		}
	}
}

func (s *RetryingChatStream) retry(cause error) error {
	err := s.ctx.Err()
	if err != nil {
		return err
	}

	if s.hasOutput || s.retries >= s.settings.MaxRetries {
		return cause
	}

	retryAfter, allowed := upstreamRateLimit(cause)
	if !allowed {
		return cause
	}

	s.Close()

	delay := max(s.settings.delay(s.retries), retryAfter)
	s.retries++

	debug("upstream rate limit: retry %d/%d after %s", s.retries, s.settings.MaxRetries, delay)

	if s.notify != nil {
		err = s.notify(RetryChunk{
			Attempt: s.retries,
			Total:   s.settings.MaxRetries,
			DelayMs: delay.Milliseconds(),
		})

		if err != nil {
			return fmt.Errorf("send retry status: %w", err)
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-timer.C:
		return nil
	}
}

func DefaultChatRetrySettings() ChatRetrySettings {
	return ChatRetrySettings{
		MaxRetries:   5,
		InitialDelay: 2,
		MaxDelay:     30,
	}
}

func upstreamRateLimit(err error) (time.Duration, bool) {
	var (
		providerError *openingrouter.ProviderError
		routerError   *openingrouter.OpenRouterError
		streamError   *openingrouter.ChatStreamError
		status        openingrouter.ErrorStatus
	)

	switch {
	case errors.As(err, &providerError):
		status = providerError.ErrorStatus
	case errors.As(err, &routerError):
		status = routerError.ErrorStatus
	case errors.As(err, &streamError):
		status.Code = streamError.Code

		if streamError.Metadata != nil {
			status.Type = openingrouter.ErrorType(streamError.Metadata.ErrorType)
		}
	default:
		return 0, false
	}

	if status.Code != http.StatusTooManyRequests {
		return 0, false
	}

	if status.Type != "" && status.Type != openingrouter.ErrorTypeRateLimitExceeded {
		return 0, false
	}

	// A generic 429 can mean a daily quota or exhausted credits, not temporary upstream throttling.
	temporary := strings.Contains(strings.ToLower(err.Error()), "temporarily rate-limited upstream")

	return status.RetryAfter, temporary
}

func completionChunkHasOutput(chunk *openingrouter.ChatStreamChunk) bool {
	if chunk.Usage != nil {
		return true
	}

	for index := range chunk.Choices {
		choice := &chunk.Choices[index]
		delta := &choice.Delta

		if choice.FinishReason != "" || delta.Content != "" || delta.Reasoning != "" || delta.Refusal != "" || delta.Audio != nil {
			return true
		}

		if len(delta.ToolCalls) > 0 || len(delta.Images) > 0 || len(delta.ReasoningDetails) > 0 {
			return true
		}
	}

	return false
}
