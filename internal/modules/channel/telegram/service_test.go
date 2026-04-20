package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

func TestSendMessageUsesTelegramAPIWhenTokenConfigured(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	var capturedBody []byte
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			capturedRequest = req
			capturedBody = body
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":12345}}`)),
			}, nil
		}),
	}

	svc := NewService(nil, Config{
		BotToken: "bot-token",
		BaseURL:  "https://telegram.example.test",
		Client:   client,
		Metrics:  foundationmetrics.New(),
	})

	result, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "telegram",
		Target:  "-1001001",
		Body:    "hello",
		Actions: []contracts.ChannelAction{
			{Label: "approve", Value: "approve:exe-1"},
		},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.MessageID != "12345" {
		t.Fatalf("unexpected message id: %s", result.MessageID)
	}
	if capturedRequest == nil {
		t.Fatalf("expected outbound request")
	}
	if capturedRequest.URL.String() != "https://telegram.example.test/botbot-token/sendMessage" {
		t.Fatalf("unexpected request url: %s", capturedRequest.URL.String())
	}
	if !bytes.Contains(capturedBody, []byte(`"chat_id":"-1001001"`)) {
		t.Fatalf("unexpected body: %s", string(capturedBody))
	}
	if !bytes.Contains(capturedBody, []byte(`"text":"hello"`)) {
		t.Fatalf("unexpected body: %s", string(capturedBody))
	}
	if !bytes.Contains(capturedBody, []byte(`"callback_data":"approve:exe-1"`)) {
		t.Fatalf("unexpected body: %s", string(capturedBody))
	}
}

func TestSendMessageFallsBackToStubWhenTokenMissing(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, Config{Metrics: foundationmetrics.New()})
	result, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "telegram",
		Target:  "-1001001",
		Body:    "hello",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.MessageID != "stub-message-id" {
		t.Fatalf("unexpected stub message id: %s", result.MessageID)
	}
}

func TestSendMessageFallsBackToStubWhenTokenIsPlaceholder(t *testing.T) {
	t.Parallel()

	called := false
	svc := NewService(nil, Config{
		BotToken: "REPLACE_WITH_TELEGRAM_BOT_TOKEN",
		Client: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				called = true
				return nil, errors.New("unexpected request")
			}),
		},
		Metrics: foundationmetrics.New(),
	})

	result, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "telegram",
		Target:  "-1001001",
		Body:    "hello",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.MessageID != "stub-message-id" {
		t.Fatalf("unexpected stub message id: %s", result.MessageID)
	}
	if called {
		t.Fatalf("unexpected network request")
	}
}

func TestNewServiceUsesDefaultClientWhenNil(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, Config{
		PollTimeout: 30 * time.Second,
	})
	if svc.client == nil {
		t.Fatalf("expected default client")
	}
	if svc.client.Timeout <= svc.cfg.PollTimeout {
		t.Fatalf("expected client timeout %s to exceed poll timeout %s", svc.client.Timeout, svc.cfg.PollTimeout)
	}
	if svc.client.Timeout < 10*time.Second {
		t.Fatalf("expected client timeout to preserve the safety minimum, got %s", svc.client.Timeout)
	}
}

func TestWithDefaultClientKeepsMinimumTimeoutForShortPolls(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		pollTimeout time.Duration
	}{
		{name: "no poll timeout", pollTimeout: 0},
		{name: "short poll timeout", pollTimeout: time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := withDefaultClient(nil, tc.pollTimeout)
			if client == nil {
				t.Fatalf("expected default client")
			}
			if client.Timeout != 10*time.Second {
				t.Fatalf("expected minimum timeout to remain 10s for poll timeout %s, got %s", tc.pollTimeout, client.Timeout)
			}
		})
	}
}

func TestAnswerCallbackQueryUsesTelegramAPIWhenTokenConfigured(t *testing.T) {
	t.Parallel()

	var capturedRequest *http.Request
	var capturedBody []byte
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			capturedRequest = req
			capturedBody = body
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
			}, nil
		}),
	}

	svc := NewService(nil, Config{
		BotToken: "bot-token",
		BaseURL:  "https://telegram.example.test",
		Client:   client,
		Metrics:  foundationmetrics.New(),
	})

	if err := svc.AnswerCallbackQuery(context.Background(), "cbq-1", "done"); err != nil {
		t.Fatalf("answer callback query: %v", err)
	}
	if capturedRequest == nil {
		t.Fatalf("expected outbound request")
	}
	if capturedRequest.URL.String() != "https://telegram.example.test/botbot-token/answerCallbackQuery" {
		t.Fatalf("unexpected request url: %s", capturedRequest.URL.String())
	}
	if !bytes.Contains(capturedBody, []byte(`"callback_query_id":"cbq-1"`)) {
		t.Fatalf("unexpected body: %s", string(capturedBody))
	}
	if !bytes.Contains(capturedBody, []byte(`"text":"done"`)) {
		t.Fatalf("unexpected body: %s", string(capturedBody))
	}
}

func TestStartPollingProcessesCallbackUpdates(t *testing.T) {
	t.Parallel()

	deleteWebhookCalls := 0
	getUpdatesCalls := 0
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/botbot-token/deleteWebhook":
				deleteWebhookCalls++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
				}, nil
			case "/botbot-token/getUpdates":
				getUpdatesCalls++
				body := `{"ok":true,"result":[{"update_id":321,"callback_query":{"id":"cbq-1","data":"approve:exe-1","from":{"id":42,"username":"alice"},"message":{"chat":{"id":"-1001001"}}}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			default:
				t.Fatalf("unexpected request path: %s", req.URL.Path)
				return nil, nil
			}
		}),
	}

	svc := NewService(nil, Config{
		BotToken:       "bot-token",
		BaseURL:        "https://telegram.example.test",
		PollingEnabled: true,
		PollTimeout:    time.Second,
		PollInterval:   time.Millisecond,
		Client:         client,
		Metrics:        foundationmetrics.New(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processed []byte
	svc.StartPolling(ctx, func(_ context.Context, rawUpdate []byte) error {
		processed = append([]byte(nil), rawUpdate...)
		cancel()
		return nil
	})

	if deleteWebhookCalls != 1 {
		t.Fatalf("expected deleteWebhook once, got %d", deleteWebhookCalls)
	}
	if getUpdatesCalls == 0 {
		t.Fatalf("expected getUpdates to be called")
	}
	if len(processed) == 0 {
		t.Fatalf("expected update to be processed")
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(processed, &decoded); err != nil {
		t.Fatalf("decode processed update: %v", err)
	}
	if decoded["update_id"].(float64) != 321 {
		t.Fatalf("unexpected update id: %+v", decoded)
	}
}

func TestStartPollingReturnsWhenDisabled(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, Config{})
	svc.StartPolling(context.Background(), func(context.Context, []byte) error {
		t.Fatalf("processor should not be called")
		return nil
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSendMessageWithAttachmentsAndActions(t *testing.T) {
	var (
		mu       sync.Mutex
		bodies   = map[string][]byte{}
		headers  = map[string]http.Header{}
		calls    = map[string]int{}
		received = map[string]struct{}{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		bodies[r.URL.Path] = append([]byte(nil), body...)
		headers[r.URL.Path] = r.Header.Clone()
		calls[r.URL.Path]++
		received[r.URL.Path] = struct{}{}
		mu.Unlock()

		switch r.URL.Path {
		case "/botbot-token/sendPhoto":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":101}}`))
		case "/botbot-token/sendDocument":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":202}}`))
		case "/botbot-token/sendMessage":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":303}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	svc := NewService(nil, Config{
		BotToken: "bot-token",
		BaseURL:  srv.URL,
		Client:   srv.Client(),
		Metrics:  foundationmetrics.New(),
	})

	result, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "telegram",
		Target:  "-1001001",
		Body:    "hello",
		Actions: []contracts.ChannelAction{
			{Label: "", Value: "skip"},
			{Label: "approve", Value: "approve:exe-1"},
		},
		Attachments: []contracts.MessageAttachment{
			{Type: "image", URL: "https://files.example/photo.png", Name: "preview"},
			{
				Type:    "document",
				Name:    "report.txt",
				Content: base64.StdEncoding.EncodeToString([]byte("payload")),
				Metadata: map[string]interface{}{
					"encoding": "base64",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.MessageID != "101" {
		t.Fatalf("unexpected message id: %s", result.MessageID)
	}

	mu.Lock()
	photoBody := append([]byte(nil), bodies["/botbot-token/sendPhoto"]...)
	documentBody := append([]byte(nil), bodies["/botbot-token/sendDocument"]...)
	messageBody := append([]byte(nil), bodies["/botbot-token/sendMessage"]...)
	documentHeader := headers["/botbot-token/sendDocument"].Clone()
	photoCalls := calls["/botbot-token/sendPhoto"]
	documentCalls := calls["/botbot-token/sendDocument"]
	messageCalls := calls["/botbot-token/sendMessage"]
	_, sawPhoto := received["/botbot-token/sendPhoto"]
	_, sawDocument := received["/botbot-token/sendDocument"]
	_, sawMessage := received["/botbot-token/sendMessage"]
	mu.Unlock()

	if !sawPhoto || photoCalls != 1 {
		t.Fatalf("expected one photo request, got %d", photoCalls)
	}
	if !sawDocument || documentCalls != 1 {
		t.Fatalf("expected one document request, got %d", documentCalls)
	}
	if !sawMessage || messageCalls != 1 {
		t.Fatalf("expected one message request, got %d", messageCalls)
	}

	var photoPayload map[string]interface{}
	if err := json.Unmarshal(photoBody, &photoPayload); err != nil {
		t.Fatalf("decode photo payload: %v", err)
	}
	if photoPayload["chat_id"] != "-1001001" {
		t.Fatalf("unexpected photo chat id: %#v", photoPayload["chat_id"])
	}
	if photoPayload["photo"] != "https://files.example/photo.png" {
		t.Fatalf("unexpected photo url: %#v", photoPayload["photo"])
	}
	if photoPayload["caption"] != "preview" {
		t.Fatalf("unexpected photo caption: %#v", photoPayload["caption"])
	}

	var messagePayload map[string]interface{}
	if err := json.Unmarshal(messageBody, &messagePayload); err != nil {
		t.Fatalf("decode message payload: %v", err)
	}
	replyMarkup, ok := messagePayload["reply_markup"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing reply markup: %#v", messagePayload["reply_markup"])
	}
	rows, ok := replyMarkup["inline_keyboard"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected inline keyboard rows: %#v", replyMarkup["inline_keyboard"])
	}
	row, ok := rows[0].([]interface{})
	if !ok || len(row) != 1 {
		t.Fatalf("unexpected inline keyboard row: %#v", rows[0])
	}
	button, ok := row[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected inline keyboard button: %#v", row[0])
	}
	if button["text"] != "approve" || button["callback_data"] != "approve:exe-1" {
		t.Fatalf("unexpected inline keyboard button: %#v", button)
	}

	mediaType, params, err := mime.ParseMediaType(documentHeader.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse multipart content type: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("unexpected multipart media type: %s", mediaType)
	}
	reader := multipart.NewReader(bytes.NewReader(documentBody), params["boundary"])
	fields := map[string]string{}
	fileName := ""
	fileField := ""
	var fileContent []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read multipart part: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read multipart body: %v", err)
		}
		if part.FileName() == "" {
			fields[part.FormName()] = string(data)
			continue
		}
		fileField = part.FormName()
		fileName = part.FileName()
		fileContent = append([]byte(nil), data...)
	}
	if fields["chat_id"] != "-1001001" {
		t.Fatalf("unexpected document chat id: %#v", fields["chat_id"])
	}
	if fields["caption"] != "report.txt" {
		t.Fatalf("unexpected document caption: %#v", fields["caption"])
	}
	if fileField != "document" {
		t.Fatalf("unexpected file field: %s", fileField)
	}
	if fileName != "report.txt" {
		t.Fatalf("unexpected file name: %s", fileName)
	}
	if string(fileContent) != "payload" {
		t.Fatalf("unexpected file content: %s", string(fileContent))
	}
}

func TestSendMessageReturnsAttachmentError(t *testing.T) {
	called := false
	svc := NewService(nil, Config{
		BotToken: "bot-token",
		BaseURL:  "https://telegram.example.test",
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				called = true
				return nil, errors.New("unexpected request")
			}),
		},
		Metrics: foundationmetrics.New(),
	})

	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "telegram",
		Target:  "-1001001",
		Body:    "hello",
		Attachments: []contracts.MessageAttachment{
			{
				Type:    "document",
				Content: "not-base64",
				Metadata: map[string]interface{}{
					"encoding": "base64",
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected attachment decode error")
	}
	if called {
		t.Fatalf("unexpected network request after attachment failure")
	}
}

func TestSendAttachmentRemoteVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		attachment  contracts.MessageAttachment
		path        string
		wantKey     string
		wantValue   string
		wantCaption bool
		messageID   int64
	}{
		{
			name: "image",
			attachment: contracts.MessageAttachment{
				Type: "image",
				URL:  "https://files.example/photo.png",
				Name: "preview",
			},
			path:        "/botbot-token/sendPhoto",
			wantKey:     "photo",
			wantValue:   "https://files.example/photo.png",
			wantCaption: true,
			messageID:   11,
		},
		{
			name: "document",
			attachment: contracts.MessageAttachment{
				Type: "document",
				URL:  "https://files.example/report.pdf",
			},
			path:        "/botbot-token/sendDocument",
			wantKey:     "document",
			wantValue:   "https://files.example/report.pdf",
			wantCaption: false,
			messageID:   12,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					http.NotFound(w, r)
					return
				}
				body, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":` + strconv.FormatInt(tc.messageID, 10) + `}}`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
				Metrics:  foundationmetrics.New(),
			})

			gotID, err := svc.sendAttachment(context.Background(), "-1001001", tc.attachment)
			if err != nil {
				t.Fatalf("send attachment: %v", err)
			}
			if gotID != strconv.FormatInt(tc.messageID, 10) {
				t.Fatalf("unexpected message id: %s", gotID)
			}

			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload["chat_id"] != "-1001001" {
				t.Fatalf("unexpected chat id: %#v", payload["chat_id"])
			}
			if payload[tc.wantKey] != tc.wantValue {
				t.Fatalf("unexpected attachment value: %#v", payload[tc.wantKey])
			}
			if tc.wantCaption {
				if payload["caption"] != tc.attachment.Name {
					t.Fatalf("unexpected caption: %#v", payload["caption"])
				}
			} else if _, ok := payload["caption"]; ok {
				t.Fatalf("unexpected caption in payload: %#v", payload["caption"])
			}
		})
	}
}

func TestSendAttachmentUploadVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		attachment   contracts.MessageAttachment
		path         string
		wantField    string
		wantFileName string
		wantContent  string
		messageID    int64
	}{
		{
			name: "document-base64",
			attachment: contracts.MessageAttachment{
				Type:    "document",
				Name:    "report.txt",
				Content: base64.StdEncoding.EncodeToString([]byte("payload")),
				Metadata: map[string]interface{}{
					"encoding": "base64",
				},
			},
			path:         "/botbot-token/sendDocument",
			wantField:    "document",
			wantFileName: "report.txt",
			wantContent:  "payload",
			messageID:    21,
		},
		{
			name: "document-unnamed",
			attachment: contracts.MessageAttachment{
				Type:    "document",
				Content: "raw-bytes",
			},
			path:         "/botbot-token/sendDocument",
			wantField:    "document",
			wantFileName: "attachment.bin",
			wantContent:  "raw-bytes",
			messageID:    23,
		},
		{
			name: "image-plain",
			attachment: contracts.MessageAttachment{
				Type:    "image",
				Content: "pixels",
			},
			path:         "/botbot-token/sendPhoto",
			wantField:    "photo",
			wantFileName: "attachment.png",
			wantContent:  "pixels",
			messageID:    22,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				body    []byte
				headers http.Header
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					http.NotFound(w, r)
					return
				}
				body, _ = io.ReadAll(r.Body)
				headers = r.Header.Clone()
				_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":` + strconv.FormatInt(tc.messageID, 10) + `}}`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
				Metrics:  foundationmetrics.New(),
			})

			gotID, err := svc.sendAttachment(context.Background(), "-1001001", tc.attachment)
			if err != nil {
				t.Fatalf("send attachment: %v", err)
			}
			if gotID != strconv.FormatInt(tc.messageID, 10) {
				t.Fatalf("unexpected message id: %s", gotID)
			}

			mediaType, params, err := mime.ParseMediaType(headers.Get("Content-Type"))
			if err != nil {
				t.Fatalf("parse multipart content type: %v", err)
			}
			if !strings.HasPrefix(mediaType, "multipart/") {
				t.Fatalf("unexpected multipart media type: %s", mediaType)
			}

			reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
			fields := map[string]string{}
			var fileField, fileName string
			var fileContent []byte
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("read multipart part: %v", err)
				}
				data, err := io.ReadAll(part)
				if err != nil {
					t.Fatalf("read multipart body: %v", err)
				}
				if part.FileName() == "" {
					fields[part.FormName()] = string(data)
					continue
				}
				fileField = part.FormName()
				fileName = part.FileName()
				fileContent = append([]byte(nil), data...)
			}
			if fields["chat_id"] != "-1001001" {
				t.Fatalf("unexpected chat id: %#v", fields["chat_id"])
			}
			if tc.attachment.Name != "" {
				if fields["caption"] != tc.attachment.Name {
					t.Fatalf("unexpected caption: %#v", fields["caption"])
				}
			} else if _, ok := fields["caption"]; ok {
				t.Fatalf("unexpected caption field: %#v", fields["caption"])
			}
			if fileField != tc.wantField {
				t.Fatalf("unexpected file field: %s", fileField)
			}
			if fileName != tc.wantFileName {
				t.Fatalf("unexpected file name: %s", fileName)
			}
			if string(fileContent) != tc.wantContent {
				t.Fatalf("unexpected file content: %s", string(fileContent))
			}
		})
	}
}

func TestAttachmentHelpers(t *testing.T) {
	t.Run("attachmentContentBytes", func(t *testing.T) {
		cases := []struct {
			name       string
			attachment contracts.MessageAttachment
			want       string
			wantNil    bool
			wantErr    bool
		}{
			{
				name: "empty",
				attachment: contracts.MessageAttachment{
					Content: "   ",
				},
				wantNil: true,
			},
			{
				name: "plain",
				attachment: contracts.MessageAttachment{
					Content: "hello",
				},
				want: "hello",
			},
			{
				name: "base64",
				attachment: contracts.MessageAttachment{
					Content: base64.StdEncoding.EncodeToString([]byte("hello")),
					Metadata: map[string]interface{}{
						"encoding": "base64",
					},
				},
				want: "hello",
			},
			{
				name: "invalid-base64",
				attachment: contracts.MessageAttachment{
					Content: "!!!",
					Metadata: map[string]interface{}{
						"encoding": "base64",
					},
				},
				wantErr: true,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := attachmentContentBytes(tc.attachment)
				if tc.wantErr {
					if err == nil {
						t.Fatalf("expected error")
					}
					return
				}
				if err != nil {
					t.Fatalf("attachment content bytes: %v", err)
				}
				if tc.wantNil {
					if got != nil {
						t.Fatalf("expected nil content, got %q", string(got))
					}
					return
				}
				if string(got) != tc.want {
					t.Fatalf("unexpected content: %q", string(got))
				}
			})
		}
	})

	t.Run("metadataString", func(t *testing.T) {
		cases := []struct {
			name     string
			metadata map[string]interface{}
			key      string
			want     string
		}{
			{name: "nil", metadata: nil, key: "encoding", want: ""},
			{name: "missing", metadata: map[string]interface{}{"other": "value"}, key: "encoding", want: ""},
			{name: "non-string", metadata: map[string]interface{}{"encoding": 42}, key: "encoding", want: ""},
			{name: "string", metadata: map[string]interface{}{"encoding": "base64"}, key: "encoding", want: "base64"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := metadataString(tc.metadata, tc.key); got != tc.want {
					t.Fatalf("unexpected metadata string: %q", got)
				}
			})
		}
	})

	t.Run("inlineKeyboard", func(t *testing.T) {
		if got := telegramInlineKeyboard(nil); got != nil {
			t.Fatalf("expected nil keyboard, got %#v", got)
		}
		if got := telegramInlineKeyboard([]contracts.ChannelAction{{Label: "", Value: "skip"}, {Label: "approve", Value: "approve:exe-1"}}); got == nil {
			t.Fatalf("expected keyboard")
		} else {
			rows := got["inline_keyboard"].([][]map[string]string)
			if len(rows) != 1 || len(rows[0]) != 1 {
				t.Fatalf("unexpected keyboard rows: %#v", rows)
			}
			if rows[0][0]["text"] != "approve" || rows[0][0]["callback_data"] != "approve:exe-1" {
				t.Fatalf("unexpected keyboard button: %#v", rows[0][0])
			}
		}
	})

	t.Run("truncateBody", func(t *testing.T) {
		if got := truncateBody("hello", 10); got != "hello" {
			t.Fatalf("unexpected short body truncation: %q", got)
		}
		if got := truncateBody("abcdefghijk", 5); got != "abcde..." {
			t.Fatalf("unexpected long body truncation: %q", got)
		}
	})

	t.Run("sleepContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if sleepContext(ctx, time.Millisecond) {
			t.Fatalf("expected canceled context to stop sleep")
		}
		if !sleepContext(context.Background(), time.Millisecond) {
			t.Fatalf("expected sleep to complete")
		}
	})
}

func TestAnswerCallbackQueryBranches(t *testing.T) {
	t.Run("blank id", func(t *testing.T) {
		called := false
		svc := NewService(nil, Config{
			BotToken: "bot-token",
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected request")
				}),
			},
			Metrics: foundationmetrics.New(),
		})
		if err := svc.AnswerCallbackQuery(context.Background(), "   ", "ignored"); err != nil {
			t.Fatalf("blank callback query id: %v", err)
		}
		if called {
			t.Fatalf("unexpected network request")
		}
	})

	t.Run("stub", func(t *testing.T) {
		called := false
		svc := NewService(nil, Config{
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected request")
				}),
			},
			Metrics: foundationmetrics.New(),
		})
		if err := svc.AnswerCallbackQuery(context.Background(), "cbq-1", "done"); err != nil {
			t.Fatalf("stub callback query: %v", err)
		}
		if called {
			t.Fatalf("unexpected network request")
		}
	})

	t.Run("success", func(t *testing.T) {
		var body []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ = io.ReadAll(r.Body)
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
			Metrics:  foundationmetrics.New(),
		})
		if err := svc.AnswerCallbackQuery(context.Background(), "cbq-1", "done"); err != nil {
			t.Fatalf("answer callback query: %v", err)
		}
		if !bytes.Contains(body, []byte(`"callback_query_id":"cbq-1"`)) {
			t.Fatalf("unexpected body: %s", string(body))
		}
		if !bytes.Contains(body, []byte(`"text":"done"`)) {
			t.Fatalf("unexpected body: %s", string(body))
		}
	})

	t.Run("decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
			Metrics:  foundationmetrics.New(),
		})
		if err := svc.AnswerCallbackQuery(context.Background(), "cbq-1", "done"); err == nil {
			t.Fatalf("expected decode error")
		}
	})

	t.Run("status error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"ok":false,"description":"nope"}`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
			Metrics:  foundationmetrics.New(),
		})
		if err := svc.AnswerCallbackQuery(context.Background(), "cbq-1", "done"); err == nil {
			t.Fatalf("expected status error")
		}
	})
}

func TestStartPollingBranches(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		called := false
		svc := NewService(nil, Config{
			PollingEnabled: false,
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected request")
				}),
			},
		})
		svc.StartPolling(context.Background(), func(context.Context, []byte) error {
			t.Fatalf("processor should not be called")
			return nil
		})
		if called {
			t.Fatalf("unexpected network request")
		}
	})

	t.Run("nil processor", func(t *testing.T) {
		called := false
		svc := NewService(nil, Config{
			PollingEnabled: true,
			BotToken:       "bot-token",
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected request")
				}),
			},
		})
		svc.StartPolling(context.Background(), nil)
		if called {
			t.Fatalf("unexpected network request")
		}
	})

	t.Run("missing token", func(t *testing.T) {
		called := false
		svc := NewService(nil, Config{
			PollingEnabled: true,
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected request")
				}),
			},
		})
		svc.StartPolling(context.Background(), func(context.Context, []byte) error {
			t.Fatalf("processor should not be called")
			return nil
		})
		if called {
			t.Fatalf("unexpected network request")
		}
	})

	t.Run("placeholder token", func(t *testing.T) {
		called := false
		svc := NewService(nil, Config{
			PollingEnabled: true,
			BotToken:       "REPLACE_WITH_TELEGRAM_BOT_TOKEN",
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected request")
				}),
			},
			Metrics: foundationmetrics.New(),
		})
		svc.StartPolling(context.Background(), func(context.Context, []byte) error {
			t.Fatalf("processor should not be called")
			return nil
		})
		if called {
			t.Fatalf("unexpected network request")
		}
	})

	t.Run("samples repeated get updates failures", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		svc := NewService(logger, Config{
			PollingEnabled: true,
			BotToken:       "bot-token",
			BaseURL:        "https://telegram.example.test",
			PollTimeout:    time.Second,
			PollInterval:   5 * time.Millisecond,
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/botbot-token/deleteWebhook":
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
						}, nil
					case "/botbot-token/getUpdates":
						return nil, errors.New("network down")
					default:
						return nil, errors.New("unexpected path")
					}
				}),
			},
			Metrics: foundationmetrics.New(),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 70*time.Millisecond)
		defer cancel()
		svc.StartPolling(ctx, func(context.Context, []byte) error { return nil })

		output := logBuf.String()
		if count := strings.Count(output, "telegram getUpdates failed"); count != 2 {
			t.Fatalf("expected sampled logs to emit first and summary only, got %d logs:\n%s", count, output)
		}
		if !strings.Contains(output, "suppressed_count=") {
			t.Fatalf("expected summary log to include suppressed_count, got:\n%s", output)
		}
	})

	t.Run("delete webhook failure continues", func(t *testing.T) {
		var (
			deleteCalls int
			getCalls    int
			processed   []byte
			mu          sync.Mutex
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/botbot-token/deleteWebhook":
				deleteCalls++
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"ok":false,"description":"delete failed"}`))
			case "/botbot-token/getUpdates":
				getCalls++
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":321,"callback_query":{"id":"cbq-1"}}]}`))
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			PollingEnabled: true,
			BotToken:       "bot-token",
			BaseURL:        srv.URL,
			PollTimeout:    time.Second,
			PollInterval:   time.Millisecond,
			Client:         srv.Client(),
			Metrics:        foundationmetrics.New(),
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			svc.StartPolling(ctx, func(_ context.Context, rawUpdate []byte) error {
				mu.Lock()
				processed = append([]byte(nil), rawUpdate...)
				mu.Unlock()
				cancel()
				return nil
			})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("polling did not stop")
		}
		mu.Lock()
		got := append([]byte(nil), processed...)
		mu.Unlock()
		if deleteCalls != 1 {
			t.Fatalf("expected deleteWebhook once, got %d", deleteCalls)
		}
		if getCalls != 1 {
			t.Fatalf("expected getUpdates once, got %d", getCalls)
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal(got, &decoded); err != nil {
			t.Fatalf("decode processed update: %v", err)
		}
		if decoded["update_id"].(float64) != 321 {
			t.Fatalf("unexpected update id: %+v", decoded)
		}
	})

	t.Run("processor error", func(t *testing.T) {
		var getCalls int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/botbot-token/deleteWebhook":
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			case "/botbot-token/getUpdates":
				getCalls++
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":42,"message":{"message_id":7}}]}`))
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			PollingEnabled: true,
			BotToken:       "bot-token",
			BaseURL:        srv.URL,
			PollTimeout:    time.Second,
			PollInterval:   time.Millisecond,
			Client:         srv.Client(),
			Metrics:        foundationmetrics.New(),
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			svc.StartPolling(ctx, func(_ context.Context, _ []byte) error {
				cancel()
				return errors.New("processor failed")
			})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("polling did not stop")
		}
		if getCalls != 1 {
			t.Fatalf("expected one getUpdates call, got %d", getCalls)
		}
	})

	t.Run("get updates error with cancellation", func(t *testing.T) {
		var (
			deleteCalls int
			getCalls    int
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/botbot-token/deleteWebhook":
				deleteCalls++
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			case "/botbot-token/getUpdates":
				getCalls++
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"ok":false,"description":"temporary failure"}`))
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			PollingEnabled: true,
			BotToken:       "bot-token",
			BaseURL:        srv.URL,
			PollTimeout:    time.Second,
			PollInterval:   100 * time.Millisecond,
			Client:         srv.Client(),
			Metrics:        foundationmetrics.New(),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()
		processorCalled := false
		done := make(chan struct{})
		go func() {
			svc.StartPolling(ctx, func(context.Context, []byte) error {
				processorCalled = true
				return nil
			})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("polling did not stop")
		}
		if deleteCalls != 1 {
			t.Fatalf("expected one deleteWebhook call, got %d", deleteCalls)
		}
		if getCalls != 1 {
			t.Fatalf("expected one getUpdates call, got %d", getCalls)
		}
		if processorCalled {
			t.Fatalf("processor should not be called")
		}
	})
}

func TestAnswerCallbackQueryFallsBackToStubWhenTokenIsPlaceholder(t *testing.T) {
	t.Parallel()

	called := false
	svc := NewService(nil, Config{
		BotToken: "REPLACE_WITH_TELEGRAM_BOT_TOKEN",
		Client: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				called = true
				return nil, errors.New("unexpected request")
			}),
		},
		Metrics: foundationmetrics.New(),
	})

	if err := svc.AnswerCallbackQuery(context.Background(), "cbq-1", "done"); err != nil {
		t.Fatalf("answer callback query: %v", err)
	}
	if called {
		t.Fatalf("unexpected network request")
	}
}

func TestPollingHelpers(t *testing.T) {
	t.Run("deleteWebhook", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			var body []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
			})
			if err := svc.deleteWebhook(context.Background()); err != nil {
				t.Fatalf("deleteWebhook: %v", err)
			}
			if !bytes.Contains(body, []byte(`"drop_pending_updates":false`)) {
				t.Fatalf("unexpected body: %s", string(body))
			}
		})

		t.Run("decode error", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`not-json`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
			})
			if err := svc.deleteWebhook(context.Background()); err == nil {
				t.Fatalf("expected decode error")
			}
		})

		t.Run("status error", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"ok":false,"description":"failed"}`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
			})
			if err := svc.deleteWebhook(context.Background()); err == nil {
				t.Fatalf("expected status error")
			}
		})
	})

	t.Run("getUpdates", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			var body []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":5,"message":{"message_id":9}}]}`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken:    "bot-token",
				BaseURL:     srv.URL,
				PollTimeout: 30 * time.Second,
				Client:      srv.Client(),
			})
			updates, err := svc.getUpdates(context.Background(), 7)
			if err != nil {
				t.Fatalf("getUpdates: %v", err)
			}
			if len(updates) != 1 || updates[0].UpdateID != 5 {
				t.Fatalf("unexpected updates: %#v", updates)
			}
			if !bytes.Contains(body, []byte(`"offset":7`)) {
				t.Fatalf("unexpected body: %s", string(body))
			}
			if !bytes.Contains(body, []byte(`"timeout":30`)) {
				t.Fatalf("unexpected body: %s", string(body))
			}
		})

		t.Run("decode error", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`not-json`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
			})
			if _, err := svc.getUpdates(context.Background(), 0); err == nil {
				t.Fatalf("expected decode error")
			}
		})

		t.Run("invalid update payload", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":"bad"}]}`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
			})
			if _, err := svc.getUpdates(context.Background(), 0); err == nil {
				t.Fatalf("expected unmarshal error")
			}
		})

		t.Run("status error", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"ok":false,"description":"failed"}`))
			}))
			t.Cleanup(srv.Close)

			svc := NewService(nil, Config{
				BotToken: "bot-token",
				BaseURL:  srv.URL,
				Client:   srv.Client(),
			})
			if _, err := svc.getUpdates(context.Background(), 0); err == nil {
				t.Fatalf("expected status error")
			}
		})
	})
}

func TestSendMessageHTTPErrorPaths(t *testing.T) {
	t.Run("decode error", func(t *testing.T) {
		var called bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			_, _ = w.Write([]byte(`not-json`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
			Metrics:  foundationmetrics.New(),
		})
		if _, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
			Channel: "telegram",
			Target:  "-1001001",
			Body:    "hello",
		}); err == nil {
			t.Fatalf("expected decode error")
		}
		if !called {
			t.Fatalf("expected sendMessage request")
		}
	})

	t.Run("status error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"ok":false,"description":"failed"}`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
			Metrics:  foundationmetrics.New(),
		})
		if _, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
			Channel: "telegram",
			Target:  "-1001001",
			Body:    "hello",
		}); err == nil {
			t.Fatalf("expected status error")
		}
	})
}

func TestSendAttachmentErrorPaths(t *testing.T) {
	t.Run("empty content", func(t *testing.T) {
		svc := NewService(nil, Config{})
		got, err := svc.sendAttachment(context.Background(), "-1001001", contracts.MessageAttachment{Type: "document"})
		if err != nil {
			t.Fatalf("send attachment: %v", err)
		}
		if got != "" {
			t.Fatalf("unexpected message id: %q", got)
		}
	})

	t.Run("remote decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
		})
		_, err := svc.sendAttachment(context.Background(), "-1001001", contracts.MessageAttachment{
			Type: "image",
			URL:  "https://files.example/photo.png",
		})
		if err == nil {
			t.Fatalf("expected decode error")
		}
	})

	t.Run("remote status error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"ok":false,"description":"failed"}`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
		})
		_, err := svc.sendAttachment(context.Background(), "-1001001", contracts.MessageAttachment{
			Type: "document",
			URL:  "https://files.example/report.pdf",
		})
		if err == nil {
			t.Fatalf("expected status error")
		}
	})

	t.Run("upload decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
		})
		_, err := svc.sendAttachment(context.Background(), "-1001001", contracts.MessageAttachment{
			Type:    "document",
			Content: "payload",
		})
		if err == nil {
			t.Fatalf("expected decode error")
		}
	})

	t.Run("upload status error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"ok":false,"description":"failed"}`))
		}))
		t.Cleanup(srv.Close)

		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  srv.URL,
			Client:   srv.Client(),
		})
		_, err := svc.sendAttachment(context.Background(), "-1001001", contracts.MessageAttachment{
			Type:    "image",
			Content: "pixels",
		})
		if err == nil {
			t.Fatalf("expected status error")
		}
	})
}

func TestTransportErrorBranches(t *testing.T) {
	t.Run("answer callback query transport error", func(t *testing.T) {
		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  "https://telegram.example.test",
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network down")
				}),
			},
			Metrics: foundationmetrics.New(),
		})
		if err := svc.AnswerCallbackQuery(context.Background(), "cbq-1", "done"); err == nil {
			t.Fatalf("expected transport error")
		}
	})

	t.Run("deleteWebhook transport error", func(t *testing.T) {
		svc := NewService(nil, Config{
			BotToken: "bot-token",
			BaseURL:  "https://telegram.example.test",
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network down")
				}),
			},
		})
		if err := svc.deleteWebhook(context.Background()); err == nil {
			t.Fatalf("expected transport error")
		}
	})

	t.Run("getUpdates transport error", func(t *testing.T) {
		svc := NewService(nil, Config{
			BotToken:    "bot-token",
			BaseURL:     "https://telegram.example.test",
			PollTimeout: 30 * time.Second,
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network down")
				}),
			},
		})
		if _, err := svc.getUpdates(context.Background(), 0); err == nil {
			t.Fatalf("expected transport error")
		}
	})

	t.Run("startPolling canceled during getUpdates", func(t *testing.T) {
		var getCalls int
		svc := NewService(nil, Config{
			PollingEnabled: true,
			BotToken:       "bot-token",
			BaseURL:        "https://telegram.example.test",
			PollTimeout:    time.Second,
			PollInterval:   time.Second,
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/botbot-token/deleteWebhook":
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
						}, nil
					case "/botbot-token/getUpdates":
						getCalls++
						<-req.Context().Done()
						return nil, req.Context().Err()
					default:
						return nil, errors.New("unexpected path")
					}
				}),
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		processorCalled := false
		done := make(chan struct{})
		go func() {
			svc.StartPolling(ctx, func(context.Context, []byte) error {
				processorCalled = true
				return nil
			})
			close(done)
		}()
		time.Sleep(25 * time.Millisecond)
		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("polling did not stop")
		}
		if getCalls != 1 {
			t.Fatalf("expected one getUpdates call, got %d", getCalls)
		}
		if processorCalled {
			t.Fatalf("processor should not be called")
		}
	})
}
