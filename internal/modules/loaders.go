package modules

import (
	"fmt"
	"strings"
	"sync"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/modules/afk"
	"github.com/ruizlenato/smudgelord/internal/modules/config"
	"github.com/ruizlenato/smudgelord/internal/modules/lastfm"
	"github.com/ruizlenato/smudgelord/internal/modules/medias"
	"github.com/ruizlenato/smudgelord/internal/modules/menu"
	"github.com/ruizlenato/smudgelord/internal/modules/misc"
	"github.com/ruizlenato/smudgelord/internal/modules/stickers"
)

var (
	packageLoadersMutex sync.Mutex
	packageLoaders      = map[string]func(*telegram.Client){
		"afk":      afk.Load,
		"config":   config.Load,
		"lastfm":   lastfm.Load,
		"medias":   medias.Load,
		"menu":     menu.Load,
		"misc":     misc.Load,
		"stickers": stickers.Load,
	}
)

func Load(client *telegram.Client) {
	var wg sync.WaitGroup

	done := make(chan struct{}, len(packageLoaders))
	moduleNames := make([]string, 0, len(packageLoaders))

	for name, loadFunc := range packageLoaders {
		wg.Add(1)

		go func(name string, loadFunc func(*telegram.Client)) {
			defer wg.Done()

			packageLoadersMutex.Lock()
			defer packageLoadersMutex.Unlock()

			loadFunc(client)

			done <- struct{}{}
			moduleNames = append(moduleNames, name)
		}(name, loadFunc)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	for range done {
	}

	joinedModuleNames := strings.Join(moduleNames, ", ")

	fmt.Printf("\033[0;35mModules Loaded:\033[0m %s\n", joinedModuleNames)
}
