// Package modules registers and loads bot feature modules.
package modules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/ruizlenato/smudgelord/internal/modules/afk"
	"github.com/ruizlenato/smudgelord/internal/modules/config"
	"github.com/ruizlenato/smudgelord/internal/modules/lastfm"
	"github.com/ruizlenato/smudgelord/internal/modules/medias"
	"github.com/ruizlenato/smudgelord/internal/modules/menu"
	"github.com/ruizlenato/smudgelord/internal/modules/misc"
	"github.com/ruizlenato/smudgelord/internal/modules/stickers"
	"github.com/ruizlenato/smudgelord/internal/modules/sudoers"
)

var packageLoaders = map[string]func(*ext.Dispatcher){
	"afk":      afk.Load,
	"config":   config.Load,
	"lastfm":   lastfm.Load,
	"medias":   medias.Load,
	"menu":     menu.Load,
	"misc":     misc.Load,
	"stickers": stickers.Load,
	"sudoers":  sudoers.Load,
}

func RegisterHandlers(dispatcher *ext.Dispatcher) {
	moduleNames := make([]string, 0, len(packageLoaders))
	for name := range packageLoaders {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	for _, name := range moduleNames {
		packageLoaders[name](dispatcher)
	}

	fmt.Printf("\033[0;35mModules Loaded:\033[0m %s\n", strings.Join(moduleNames, ", "))
}
