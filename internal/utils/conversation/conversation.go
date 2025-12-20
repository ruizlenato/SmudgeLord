package conversation

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	ErrConversationTimeout  = fmt.Errorf("conversation timed out")
	ErrConversationAborted  = fmt.Errorf("conversation aborted by user")
	ErrConversationCanceled = fmt.Errorf("conversation canceled")
)

type Manager struct {
	b                   *bot.Bot
	activeConversations map[int64]*Conversation
	mu                  sync.RWMutex
}

func NewManager(b *bot.Bot) *Manager {
	return &Manager{
		b:                   b,
		activeConversations: make(map[int64]*Conversation),
	}
}

type Conversation struct {
	b             *bot.Bot
	chatID        int64
	userID        int64
	timeout       time.Duration
	abortKeywords []string
	canceled      bool
	lastMessageID int
	manager       *Manager
}

type ConversationOptions struct {
	Timeout       time.Duration
	AbortKeywords []string
}

func (m *Manager) Start(chatID, userID int64, opts ...*ConversationOptions) *Conversation {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existingConv, exists := m.activeConversations[userID]; exists {
		existingConv.cancel()
	}

	options := &ConversationOptions{
		Timeout: 60 * time.Second,
	}

	if len(opts) > 0 && opts[0] != nil {
		if opts[0].Timeout > 0 {
			options.Timeout = opts[0].Timeout
		}
		if len(opts[0].AbortKeywords) > 0 {
			options.AbortKeywords = opts[0].AbortKeywords
		}
	}

	conv := &Conversation{
		b:             m.b,
		chatID:        chatID,
		userID:        userID,
		timeout:       options.Timeout,
		abortKeywords: options.AbortKeywords,
		canceled:      false,
		manager:       m,
	}

	m.activeConversations[userID] = conv

	return conv
}

func (c *Conversation) cancel() {
	c.canceled = true
}

func (c *Conversation) Cancel() {
	c.cancel()
	c.removeFromManager()
}

func (c *Conversation) IsCanceled() bool {
	return c.canceled
}

func (c *Conversation) removeFromManager() {
	if c.manager != nil {
		c.manager.mu.Lock()
		defer c.manager.mu.Unlock()
		delete(c.manager.activeConversations, c.userID)
	}
}

func (c *Conversation) End() {
	c.removeFromManager()
}

func (c *Conversation) checkAbort(msg *models.Message) bool {
	if len(c.abortKeywords) == 0 {
		return false
	}
	text := msg.Text
	return slices.Contains(c.abortKeywords, text)
}

func (c *Conversation) AwaitResponse(ctx context.Context) (*models.Message, error) {
	if c.canceled {
		return nil, ErrConversationCanceled
	}

	responseChan := make(chan *models.Message, 1)

	match := func(update *models.Update) bool {
		return update.Message != nil &&
			update.Message.Chat.ID == c.chatID &&
			update.Message.From.ID == c.userID
	}

	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		select {
		case responseChan <- update.Message:
		default:
		}
	}

	handlerID := c.b.RegisterHandlerMatchFunc(match, handler)

	defer c.b.UnregisterHandler(handlerID)

	select {
	case msg := <-responseChan:
		if c.canceled {
			return nil, ErrConversationCanceled
		}
		return msg, nil
	case <-time.After(c.timeout):
		return nil, ErrConversationTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Conversation) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	if c.canceled {
		return nil, ErrConversationCanceled
	}

	params.ChatID = c.chatID
	msg, err := c.b.SendMessage(ctx, params)
	if err == nil && msg != nil {
		c.lastMessageID = msg.ID
	}
	return msg, err
}

func (c *Conversation) GetLastMessageID() int {
	return c.lastMessageID
}

func (c *Conversation) Ask(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	if c.canceled {
		return nil, ErrConversationCanceled
	}

	if _, err := c.SendMessage(ctx, params); err != nil {
		return nil, fmt.Errorf("sending message: %w", err)
	}

	msg, err := c.AwaitResponse(ctx)
	if err != nil {
		return nil, err
	}

	if c.checkAbort(msg) {
		c.Cancel()
		return nil, ErrConversationAborted
	}

	return msg, nil
}
