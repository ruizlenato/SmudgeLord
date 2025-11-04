package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/localization"
	tg "github.com/ruizlenato/smudgelord/internal/telegram"
)

func initializeServices() error {
	if err := localization.LoadLanguages(); err != nil {
		return fmt.Errorf("load languages: %v", err)
	}

	if err := database.Open(config.DatabaseFile); err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err := database.CreateTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	if err := cache.ValkeyClient("localhost:6379"); err != nil {
		fmt.Println("\033[0;31mValkey cache is currently unavailable.\033[0m")
	}

	return nil
}

type colorHandler struct {
	handler slog.Handler
	out     io.Writer
	opts    *slog.HandlerOptions
	colors  map[slog.Level]string

	sendQueue chan string
}

var (
	globalLogQueue chan string
	onceStartQueue sync.Once
)

func startGlobalLogSender() {
	onceStartQueue.Do(func() {
		globalLogQueue = make(chan string, 200)
		go func() {
			for msg := range globalLogQueue {
				_, err := tg.Client.SendMessage(config.LogChannelID, msg, &telegram.SendOptions{
					ParseMode: telegram.HTML,
				})
				if err != nil {
					fmt.Fprintln(os.Stderr, "\033[0;31mFailed to send log message to Telegram:\033[0m", err)
				}
			}
		}()
	})
}

func newColorHandler(out io.Writer, opts *slog.HandlerOptions) *colorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	startGlobalLogSender()

	return &colorHandler{
		handler:   slog.NewTextHandler(out, opts),
		out:       out,
		opts:      opts,
		sendQueue: globalLogQueue,
		colors: map[slog.Level]string{
			slog.LevelError: "\033[0;31m", // red
			slog.LevelWarn:  "\033[0;33m", // yellow
			slog.LevelInfo:  "\033[0;36m", // cyan
			slog.LevelDebug: "\033[0;32m", // green
		},
	}
}

func (h *colorHandler) Handle(ctx context.Context, r slog.Record) error {
	timestamp := r.Time.Format("[01/02 15:04]")
	colorCode, ok := h.colors[r.Level]
	if !ok {
		colorCode = "\033[0m"
	}

	colorReset := "\033[0m"
	colorGray := "\033[90m"
	colorWhiteBold := "\033[1;37m"

	attrs := make(map[string]any)
	if h.opts.AddSource {
		if pc := r.PC; pc != 0 {
			fs := runtime.Frame{}
			frames := runtime.CallersFrames([]uintptr{pc})
			if frame, _ := frames.Next(); frame != (runtime.Frame{}) {
				fs = runtime.Frame{
					File: frame.File,
					Line: frame.Line,
				}
			}

			file := fs.File
			if wd, err := os.Getwd(); err == nil {
				if rel, err := filepath.Rel(filepath.Dir(wd), file); err == nil {
					file = "./" + rel
				}
			}
			attrs["Source"] = file + ":" + strconv.Itoa(fs.Line)
		}
	}

	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "" {
			attrs[a.Key] = a.Value.Any()
		}
		return true
	})

	var jsonAttrs string
	if len(attrs) > 0 {
		jsonBytes, err := json.MarshalIndent(attrs, "", "  ")
		if err == nil {
			jsonAttrs = " " + string(jsonBytes)
		}
	}

	msg := fmt.Sprintf("%s%s %s%s%s: %s%s%s\n",
		colorGray,
		timestamp,
		colorCode,
		r.Level.String(),
		colorWhiteBold,
		r.Message,
		colorReset,
		jsonAttrs,
	)

	htmlContent := html.EscapeString(fmt.Sprintf("%s %s: %s%s", timestamp, r.Level.String(), r.Message, jsonAttrs))
	htmlMsg := fmt.Sprintf(`<pre language="json">%s</pre>`, htmlContent)

	select {
	case h.sendQueue <- htmlMsg:
	default:
		fmt.Fprintln(os.Stderr, "\033[0;33mLog queue full: dropping log message\033[0m")
	}

	_, err := h.out.Write([]byte(msg))
	return err
}

func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &colorHandler{
		handler:   h.handler.WithAttrs(attrs),
		out:       h.out,
		opts:      h.opts,
		colors:    h.colors,
		sendQueue: h.sendQueue,
	}
}

func (h *colorHandler) WithGroup(name string) slog.Handler {
	return &colorHandler{
		handler:   h.handler.WithGroup(name),
		out:       h.out,
		opts:      h.opts,
		colors:    h.colors,
		sendQueue: h.sendQueue,
	}
}

func (h *colorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}
