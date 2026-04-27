package conversation

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var (
	ErrConversationTimeout  = fmt.Errorf("conversation timed out")
	ErrConversationAborted  = fmt.Errorf("conversation aborted by user")
	ErrConversationCanceled = fmt.Errorf("conversation canceled")

	shutdownCtx context.Context = context.Background()
	shutdownMu  sync.Mutex
)

const conversationHandlerGroup = -100

func SetShutdownContext(ctx context.Context) {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	shutdownCtx = ctx
}

func getShutdownContext() context.Context {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	return shutdownCtx
}

type Manager struct {
	b                   *gotgbot.Bot
	d                   *ext.Dispatcher
	activeConversations map[int64]*Conversation
	mu                  sync.RWMutex
}

func NewManager(b *gotgbot.Bot, dispatcher *ext.Dispatcher) *Manager {
	return &Manager{
		b:                   b,
		d:                   dispatcher,
		activeConversations: make(map[int64]*Conversation),
	}
}

type Conversation struct {
	b             *gotgbot.Bot
	d             *ext.Dispatcher
	chatID        int64
	userID        int64
	timeout       time.Duration
	abortKeywords []string
	canceled      bool
	lastMessageID int64
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

	options := &ConversationOptions{Timeout: 60 * time.Second}
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
		d:             m.d,
		chatID:        chatID,
		userID:        userID,
		timeout:       options.Timeout,
		abortKeywords: options.AbortKeywords,
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

func (c *Conversation) checkAbort(msg *gotgbot.Message) bool {
	if len(c.abortKeywords) == 0 {
		return false
	}
	return slices.Contains(c.abortKeywords, msg.GetText())
}

func (c *Conversation) AwaitResponse(ctx context.Context) (*gotgbot.Message, error) {
	if c.canceled {
		return nil, ErrConversationCanceled
	}

	if c.d == nil {
		return nil, fmt.Errorf("dispatcher is required for conversations")
	}

	responseChan := make(chan *gotgbot.Message, 1)
	handlerName := fmt.Sprintf("conversation_%d_%d_%d", c.chatID, c.userID, time.Now().UnixNano())

	h := conversationHandler{
		name: handlerName,
		check: func(_ *gotgbot.Bot, ectx *ext.Context) bool {
			return ectx.Message != nil && ectx.Message.Chat.Id == c.chatID && ectx.Message.From != nil && ectx.Message.From.Id == c.userID
		},
		handle: func(_ *gotgbot.Bot, ectx *ext.Context) error {
			select {
			case responseChan <- ectx.Message:
			default:
			}
			return nil
		},
	}

	c.d.AddHandlerToGroup(h, conversationHandlerGroup)
	defer c.d.RemoveHandlerFromGroup(handlerName, conversationHandlerGroup)

	timer := time.NewTimer(c.timeout)
	defer timer.Stop()

	select {
	case msg := <-responseChan:
		if c.canceled {
			return nil, ErrConversationCanceled
		}
		return msg, nil
	case <-timer.C:
		return nil, ErrConversationTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-getShutdownContext().Done():
		return nil, ErrConversationCanceled
	}
}

func (c *Conversation) SendMessage(_ context.Context, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	if c.canceled {
		return nil, ErrConversationCanceled
	}

	msg, err := c.b.SendMessage(c.chatID, text, opts)
	if err == nil && msg != nil {
		c.lastMessageID = msg.MessageId
	}
	return msg, err
}

func (c *Conversation) GetLastMessageID() int64 {
	return c.lastMessageID
}

func (c *Conversation) Ask(ctx context.Context, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	if c.canceled {
		return nil, ErrConversationCanceled
	}

	if _, err := c.SendMessage(ctx, text, opts); err != nil {
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

type conversationHandler struct {
	name   string
	check  func(*gotgbot.Bot, *ext.Context) bool
	handle func(*gotgbot.Bot, *ext.Context) error
}

func (h conversationHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	return h.check != nil && h.check(b, ctx)
}

func (h conversationHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	if h.handle == nil {
		return nil
	}
	return h.handle(b, ctx)
}

func (h conversationHandler) Name() string {
	return h.name
}
