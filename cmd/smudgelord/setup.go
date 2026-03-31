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
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/localization"
)

func initializeServices(b *gotgbot.Bot, ctx context.Context) error {
	slog.Info("loading languages")
	if err := localization.LoadLanguages(); err != nil {
		return fmt.Errorf("load languages: %w", err)
	}
	slog.Info("languages loaded", "count", len(database.AvailableLocales))

	slog.Info("opening database", "path", config.DatabaseFile)
	if err := database.Open(config.DatabaseFile); err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	slog.Info("creating database tables")
	if err := database.CreateTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	slog.Info("database ready")

	if err := cache.RedisClient("localhost:6379", "", 0); err != nil {
		fmt.Println("\033[0;31mRedis cache is currently unavailable.\033[0m")
		slog.Warn("redis cache unavailable", "error", err)
	} else {
		slog.Info("redis cache connected")
	}

	slog.Info("services initialized")
	startDatabaseBackupRoutine(ctx, b)
	return nil
}

type ColorHandler struct {
	handler slog.Handler
	out     io.Writer
	colors  map[slog.Level]string
	opts    *slog.HandlerOptions
	b       *gotgbot.Bot
	chatID  int64
}

func NewColorHandler(out io.Writer, opts *slog.HandlerOptions, b *gotgbot.Bot, chatID int64) *ColorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	return &ColorHandler{
		handler: slog.NewTextHandler(out, opts),
		out:     out,
		opts:    opts,
		b:       b,
		chatID:  chatID,
		colors: map[slog.Level]string{
			slog.LevelError: "\033[0;31m",
			slog.LevelWarn:  "\033[0;33m",
			slog.LevelInfo:  "\033[0;36m",
			slog.LevelDebug: "\033[0;32m",
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
				fs = runtime.Frame{File: frame.File, Line: frame.Line}
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
			jsonAttrs = string(jsonBytes)
		}
	}

	terminalAttrs := ""
	if jsonAttrs != "" {
		terminalAttrs = " " + jsonAttrs
	}

	msg := fmt.Sprintf("%s%s %s%s%s: %s%s%s\n",
		colorGray,
		timestamp,
		colorCode,
		r.Level.String(),
		colorWhiteBold,
		r.Message,
		colorReset,
		terminalAttrs,
	)

	_, err := h.out.Write([]byte(msg))
	h.sendToTelegram(r, jsonAttrs)
	return err
}

func (h *ColorHandler) sendToTelegram(r slog.Record, jsonAttrs string) {
	if h.b == nil || h.chatID == 0 {
		return
	}

	if r.Level == slog.LevelDebug || r.Level == slog.LevelInfo {
		return
	}

	message := fmt.Sprintf("<b>%s</b>: %s", strings.ToUpper(r.Level.String()), html.EscapeString(r.Message))
	if jsonAttrs != "" {
		message += "\n<pre><code class=\"language-json\">" + html.EscapeString(jsonAttrs) + "</code></pre>"
	}
	if len(message) > 4096 {
		message = message[:4093] + "..."
	}

	go func() {
		_, err := h.b.SendMessage(h.chatID, message, &gotgbot.SendMessageOpts{
			ParseMode:   gotgbot.ParseModeHTML,
			RequestOpts: &gotgbot.RequestOpts{Timeout: 5 * time.Second},
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to send slog to telegram: %v\n", err)
		}
	}()
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithAttrs(attrs),
		out:     h.out,
		opts:    h.opts,
		b:       h.b,
		chatID:  h.chatID,
		colors:  h.colors,
	}
}

func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{
		handler: h.handler.WithGroup(name),
		out:     h.out,
		opts:    h.opts,
		b:       h.b,
		chatID:  h.chatID,
		colors:  h.colors,
	}
}

func (h *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func startDatabaseBackupRoutine(ctx context.Context, b *gotgbot.Bot) {
	if b == nil || config.LogChannelID == 0 || config.DatabaseFile == "" {
		return
	}

	ticker := time.NewTicker(time.Hour)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := sendDatabaseBackup(b); err != nil {
					slog.Error("failed to send database backup", "error", err.Error())
				}
			}
		}
	}()
}

func sendDatabaseBackup(b *gotgbot.Bot) error {
	backupPath, err := database.CreateBackupFile()
	if err != nil {
		return err
	}

	defer func() {
		if err := os.Remove(backupPath); err != nil {
			slog.Warn("failed to remove temporary backup file", "path", backupPath, "error", err.Error())
		}
	}()

	backupFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer backupFile.Close()

	_, err = b.SendDocument(config.LogChannelID, gotgbot.InputFileByReader(filepath.Base(backupPath), backupFile), &gotgbot.SendDocumentOpts{
		Caption:   "<b>DATABASE BACKUP</b>",
		ParseMode: gotgbot.ParseModeHTML,
		RequestOpts: &gotgbot.RequestOpts{
			Timeout: 30 * time.Second,
		},
	})
	if err != nil {
		return fmt.Errorf("send backup to telegram: %w", err)
	}

	return nil
}
