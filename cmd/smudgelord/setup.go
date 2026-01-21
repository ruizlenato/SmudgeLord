package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/localization"
)

func InitializeServices(b *bot.Bot, ctx context.Context) error {
	if err := localization.LoadLanguages(); err != nil {
		return fmt.Errorf("load languages: %w", err)
	}

	if err := database.Open(config.DatabaseFile); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := database.CreateTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err := cache.RedisClient("localhost:6379", "", 0); err != nil {
		fmt.Println("\033[0;31mRedis cache is currently unavailable.\033[0m")
	}

	if config.WebhookURL != "" {
		_, err := b.SetWebhook(ctx, &bot.SetWebhookParams{
			URL: config.WebhookURL,
		})
		if err != nil {
			return fmt.Errorf("failed to set webhook: %w", err)
		}

		go func() {
			http.ListenAndServe(":"+strconv.Itoa(config.WebhookPort), b.WebhookHandler())
		}()
	} else {
		_, err := b.DeleteWebhook(ctx, &bot.DeleteWebhookParams{
			DropPendingUpdates: true,
		})
		if err != nil {
			return fmt.Errorf("failed to delete webhook: %w", err)
		}
	}

	return nil
}

type ColorHandler struct {
	handler slog.Handler
	out     io.Writer
	colors  map[slog.Level]string
	opts    *slog.HandlerOptions
}

func NewColorHandler(out io.Writer, opts *slog.HandlerOptions) *ColorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	return &ColorHandler{
		handler: slog.NewTextHandler(out, opts),
		out:     out,
		opts:    opts,
		colors: map[slog.Level]string{
			slog.LevelError: "\033[0;31m", // red
			slog.LevelWarn:  "\033[0;33m", // yellow
			slog.LevelInfo:  "\033[0;36m", // cyan
			slog.LevelDebug: "\033[0;32m", // green
		},
	}
}

func (h *ColorHandler) Handle(ctx context.Context, r slog.Record) error {
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
	if r.NumAttrs() > 0 {
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

	_, err := h.out.Write([]byte(msg))
	return err
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithAttrs(attrs),
		out:     h.out,
		opts:    h.opts,
		colors:  h.colors,
	}
}

func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithGroup(name),
		out:     h.out,
		opts:    h.opts,
		colors:  h.colors,
	}
}

func (h *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}
