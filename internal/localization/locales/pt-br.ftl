language-name = Português (BR)
language-flag = 🇧🇷
language-menu =
    <b>Idioma atual:</b> { language-flag } { language-name }

    <b>Selecione abaixo o idioma que você quer utilizar no bot.</b>
language-changed = O idioma foi alterado com sucesso.
measurement-unit = m
start-button = Inciar uma conversa.
start-message =
    Olá <b>{ $userFirstName }</b> — Eu sou o <b>{ $botName }</b>, um bot com alguns comandos úteis e divertidos para você.

    <b>Código Fonte:</b> <a href='github.com/ruizlenato/smudgelord'>GitHub</a>
start-message-group =
    Olá, eu sou o <b>{ $botName }</b>
    Tenho várias funções interessantes. Para saber mais, clique no botão abaixo e inicie uma conversa comigo.

# Buttons
about-button =  ℹ️ Sobre
language-button = Idioma
help-button = ❔Ajuda
donation-button = 💵 Donation
news-channel-button = 📢 Canal
about-your-data-button = Sobre seus dados
back-button = ↩️ Voltar
denied-button-alert = Este botão não é para você.
privacy-policy-button = 🔒 Política de Privacidade
privacy-policy-group = Para acessar a política de privacidade do SmudgeLord, <b>clique no botão abaixo.</b>
about-message =
    <b>— SmudgeLord</b>
    SmudgeLord (Smudge The Cat) é um gato que se tornou um famoso meme na Internet. A sua veio de uma imagem onde aparece ao lado de uma mulher gritando com raiva.

    <b>- Código Fonte:</b> <a href='https://github.com/ruizlenato/SmudgeLord'>GitHub</a>
    <b>- Desenvolvedor:</b> @ruizlenato
    <i>Este projeto não tem afiliação com Smudge The Cat. Estou apenas usando seu nome e imagem porque sou um grande fã.</i>

    <b>💸 Contribua: Ajude a manter o bot no ar com uma doação via PIX ou PayPal.</b>
    • Chave Pix e Email do PayPal: <code>ruizlenato@proton.me</code>

    Se preferir contribuir de outra forma, como com cartão de crédito ou débito, toque no botão abaixo para ser redirecionado ao link de doação no Ko-Fi.
privacy-policy-private =
    <b>Política de Privacidade do SmudgeLord.</b>

    O SmudgeLord foi criado com o compromisso de garantir transparência e confiança aos seus usuários. 
    Agradeço pela sua confiança e estou dedicado a proteger sua privacidade.

    Esta política de privacidade pode ser atualizada, e quaisquer alterações serão informadas através do Canal do SmudgeLord - @SmudgeLordChannel.
about-your-data = 
    <b>Sobre seus dados.</b>

    <b>1. Coleta de Dados.</b>
    O bot coleta apenas informações essenciais para proporcionar uma experiência personalizada.
    <b>Os dados coletados incluem:</b>
    - <b>Informações do usuário no Telegram:</b> ID do usuário, primeiro nome, idioma e nome de usuário.
    - <b>Suas configurações no SmudgeLord:</b> Configurações que você configurou no bot, como seu idioma e nome de usuário do LastFM, tudo fornecido pelo próprio usuário.

    <b>2. Uso de dados.</b>
    Os dados coletados pelo bot são utilizados exclusivamente para aprimorar a experiência do usuário e prestar um serviço mais eficiente.
    - <b>Suas informações de usuário do Telegram</b> são usadas para identificação e comunicação com o usuário.
    - <b>Suas configurações no SmudgeLord</b> são usadas para integrar e personalizar os serviços do bot.

    <b>3. Compartilhamento de Dados.</b>
    Os dados coletados pelo bot não são compartilhados com terceiros, exceto quando exigido por lei. 
    Todos os seus dados são armazenados de forma segura.

    <b>Observação:</b> Suas informações de usuário do Telegram são informações públicas da plataforma. Não sabemos seus dados "reais".
help-message =
    Aqui estão todos os meus módulos.
    <b>Para saber mais sobre os módulos, basta clicar em seus nomes.</b>

    <b>Observação:</b>
    Alguns módulos possuem configurações adicionais em grupos.
    Para saber mais, envie <code>/config</code> em um grupo onde você é administrador.
relative-duration-seconds = { $count ->
    [one] { $count } segundo
    *[other] { $count } segundos
}
relative-duration-minutes = { $count ->
    [one] { $count } minuto
    *[other] { $count } minutos
}
relative-duration-hours = { $count ->
    [one] { $count } hora
    *[other] { $count } horas
}
relative-duration-days = { $count ->
    [one] { $count } dia
    *[other] { $count } dias
}
relative-duration-weeks = { $count ->
    [one] { $count } semana
    *[other] { $count } semanas
}
relative-duration-months = { $count ->
    [one] { $count } mês
    *[other] { $count } meses
}
afk = AFK
afk-help = 
    <b>AFK — <i>Away from Keyboard</i></b>

    <b>AFK</b> significa <b>"Longe do Teclado"</b> em português.
    É uma gíria da internet para informar que você está ausente.

    <b>— Comandos:</b>
    <b>/afk (motivo):</b> Marca você como ausente.
    <b>brb (motivo):</b> Funciona como o comando afk, mas não é necessário usar o <code>/</code>.
user-now-unavailable = <b>{ $userFirstName }</b> está agora indisponível!
user-unavailable =
    <b><a href='tg://user?id={ $userID }'>{ $userFirstName } </a></b> está <b>indisponível!</b>
    Visto pela última vez à <code>{ $duration}</code> atrás.
user-unavailable-reason = <b>Reason:</b> <code>{ $reason }</code>
user-now-available = <b><a href='tg://user?id={ $userID }'>{ $userFirstName }</a></b> está de volta após <code>{ $duration }</code> de ausência!
config = Configurações
config-help =
    <b>Configurações:</b>

    Esse módulo é feito para ser <b>utilizado em grupos.</b>
    Você deve ser administrador para utilizá-lo.

    <b>— Comandos:</b>
    <b>/disable (comando):</b> Desativa o comando especificado no grupo.
    <b>/enable (comando):</b> Reativa o comando que foi previamente desativado.
    <b>/disableable:</b> Lista todos os comandos que podem ser desativados.
    <b>/disabled:</b> Exibe os comandos que estão atualmente desativados.
    <b>/config:</b> Abre um menu com opções de configurações do grupo.
config-message =
    <b>Configurações —</b> Aqui estão minhas configurações para esse grupo.
    Para saber mais, <b>clique nos botões abaixo.</b>
config-medias =
    <b>Configurações do módulo de mídias:</b>
    Para saber mais sobre o módulo <b><i>mídias</i></b>, use /help no meu chat privado.

    <b>Para saber mais sobre cada configuração, clique em seu nome.</b>
    <i>Essas configurações são específicas para este grupo, não se aplicam a outros grupos ou chats privados.</i>
caption-button = Legendas:
automatic-button = Automático:
command-enabled = O comando <code>{ $command }</code> <b>foi ativado com sucesso.</b>
command-already-enabled = O comando <code>{ $command }</code> <b>já está ativado.</b>
enable-commands-usage =
    Especifique o comando que você deseja ativar. Para ver quais os comandos que estão atualmente desativados, utilize /disabled.

    <b>Uso:</b> <code>/enable (comando)</code>
no-disabled-commands = Não há comandos desativados <b>neste grupo.</b>
disabled-commands = <b>Comandos desativados:</b>
disableables-commands = <b>Comandos desativáveis:</b>
command-already-disabled = O comando <code>{ $command }</code> <b>já está desativado.</b>
command-disabled = O comando <code>{ $command }</code> <b>já foi desativado com sucesso.</b>
disable-commands-usage =
    Especifique o comando que você deseja desativar. Para ver a lista de comandos desativáveis, utilize /disableable.

    <b>Uso:</b> <code>/disable (comando)</code>
command-not-deactivatable = O comando <code>{ $command }</code> <b>não pode ser desativado.</b>
medias = Mídias
medias-help =
    <b>Media Downloader</b>

    Ao compartilhar links no Telegram, alguns sites não exibem uma pré-visualização de imagem ou vídeo. 
    Esse módulo faz com que o Smudge detecte automaticamente os links dos sites suportados e envie os vídeos e imagens que estão presentes no mesmo.

    <b>Sites atualmente suportados:</b> <i>Instagram</i>, <i>TikTok</i>, <i>Twitter/X</i>, <i>Threads</i>, <i>Reddit</i>, <i>Bluesky</i> e <i>YouTube Shorts</i>.

    <b>Observação:</b> 
    Esse módulo contém configurações adicionais para grupos. 
    Você pode desativar os downloads automáticos e as legendas em grupos.

    <b>— Comandos:</b>
    <b>/dl | /sdl (link):</b> Este comando é para quando você desabilita downloads automáticos em grupos.
    <b>/ytdl (link)</b>: Baixa vídeos do <b>YouTube</b>
    A qualidade máxima dos downloads de vídeo é 1080p. Você também pode baixar apenas o áudio do vídeo com este comando.
youtube-no-url =
    Você precisa especificar um link válido do YouTube para fazer o download.

    <b>Exemplo:</b> <code>/ytdl https://youtu.be/OjNpRbNdR7E</code>
youtube-invalid-url = Este link do YouTube é inválido ou é de um vídeo privado.
youtube-video-info =
    <b>Título:</b> { $title }
    <b>Autor:</b> { $author }
    <b>Tamanho:</b> <code>{ $audioSize }</code> (Áudio) | <code>{ $videoSize }</code> (Vídeo)
    <b>Duração:</b> { $duration }
youtube-download-video-button = Baixar vídeo
youtube-download-audio-button = Baixar áudio
video-exceeds-limit = 
    O vídeo excede o limite de { $size ->
       [1572864000] 1,5GB
       *[other] 50 MB
   }, meu máximo permitido.
downloading = Baixando...
uploading = Enviando...
youtube-error =
    <b>Ocorreu um erro ao processar o vídeo. Tente novamente mais tarde.</b>
    Se o problema persistir, entre em contato com meu desenvolvedor.
auto-help = Quando ativado, o bot baixará automaticamente fotos e vídeos de determinadas redes sociais ao detectar um link, eliminando a necessidade do comando /sdl ou /dl.
caption-help = Quando ativado, a legenda das mídias baixada pelo bot serão enviadas junto com a mídia.
no-link-provided =
    <b>Você não especificou um link ou especificou um link invalido.</b>
    Especifique um link do <b><i>Instagram</i></b>, <b><i>TikTok</i></b>, <b><i>Twitter/X</i></b>, <b><i>Threads</i></b>, <b><i>Reddit</i></b>, <b><i>Bluesky</i></b> ou <b><i>YouTube Shorts</i></b> para que eu possa baixar a(s) mídia(s).
misc = Diversos
misc-help =
    <b>Miscellaneous</b>

    Esse módulo reúne alguns comandos úteis que não se encaixam em outras categorias específicas.

    <b>— Comandos:</b>
    <b>/clima (cidade):</b> Exibe o clima atual da cidade especifica.
    <b>/tr (origem)-(destino) (texto):</b> Traduz um texto do idioma de origem para o idioma de destino especificado.
    <i>Caso você não especifique o idioma de origem, o Smudge irá identificar automaticamente.</i>
        

    <b>Observação:</b>
    Você pode traduzir mensagens respondendo a elas com <code>/tr</code>.
    Ambos os comandos <code>/tr</code> e <code>/translate</code> funcionam da mesma forma.
translator-no-args-provided =
    Você precisa especificar o texto que deseja traduzir ou responder a uma mensagem de texto, ou uma foto com legenda.

    <b>Usage:</b> <code>/tr (?idioma) (texto para tradução)</code>
translator-error =
    <b>Ocorreu um erro ao traduzir o texto.</b>
    Se o erro persistir, entre em contato com o meu desenvolvedor.
weather-no-location-provided =
    Você precisa especificar o local para o qual deseja saber as informações meteorológicas.
    
    <b>Exemplo:</b> <code>/clima Belém</code>.
weather-select-location = <b>Selecione o local que você deseja saber o clima:</b>
weather-details =
    <b>{ $localname }</b>:

    Temperatura: <code>{ $temperature } °C</code>
    Sensação térmica: <code>{ $temperatureFeelsLike } °C</code>
    Umidade do ar: <code>{ $relativeHumidity }%</code>
    Velocidade do vento: <code>{ $windSpeed } km/h</code>
stickers = Figurinhas
stealing-sticker = <code>Kangando (roubando) a figurinha...</code>
kang-no-reply-provided = Você precisa usar este comando respondendo a <i><b>uma figurinha</b></i>, <i><b>uma imagem</b></i> ou <i><b>um vídeo</b></i>.
converting-video-to-sticker = <code>Convertendo vídeo/gif para figurinha de vídeo...</code>
sticker-pack-already-exists = <code>Usando um pacote de figurinhas existente...</code>
kang-error =
    <b>Ocorreu um erro ao processar a figurinha, tente novamente.</b>
    Se o erro persistir, entre em contato com o meu desenvolvedor.
get-sticker-no-reply-provided =
    Você precisa usar este comando respondendo a uma <b>figurinha estática (imagem) ou de vídeo.
sticker-invalid-media-type = 
    O arquivo que você respondeu não é valido.
    Você precisa responder a uma <b>figurinha</b> (sticker), <b>vídeo</b> ou <b>foto</b>.
sticker-new-pack = <code>Criando um novo pacote de figurinhas...</code>
sticker-stoled = 
    Figurinha roubada <b>com sucesso</b>, <a href='t.me/addstickers/{ $stickerSetName }'>confira.</a>
    <b>Emoji:</b> { $emoji }
stickers-help = 
    <b>Figurinhas — Stickers</b>

    Esse módulo contém algumas funções úteis para você gerenciar figurinhas (stickers).

    <b>— Comandos:</b>
    <b>/kang (emoji):</b> Responda a qualquer figurinha para adicioná-la ao seu pacote de figurinhas criado por mim. <b>Funciona com figurinha <i>estáticas, de vídeo e animadas.</i></b>
    <b>/getsticker:</b> Responda a uma figurinha para que eu possa enviá-la como arquivo <i>.png</i> ou <i>.gif</i>. <b>Funciona apenas com figurinhas <i>de vídeo ou estáticas.</i></b>
lastfm = Last.FM
no-lastfm-username-provided =
    Você precisa especificar seu nome de usuário last.fm para que eu possa salvar meu banco de dados.
    
    <b>Examplo:</b> <code>/setuser maozedong</code>.
invalid-lastfm-username =
    <b>Usuário do last.fm inválido</b>
    Verifique se você digitou corretamente seu nome de usuário last.FM e tente novamente.
lastfm-username-not-defined =
    <b>Você ainda não definiu seu nome de usuário do last.fm.</b>
    Use o comando /setuser para definir.
lastfm-username-saved = <b>Pronto</b>, seu nome de usuário do last.fm foi salvo!
lastfm-error =
    <b>Parece que ocorreu um erro.</b>
    O last.fm pode estar temporariamente indisponível.

    Tente novamente mais tarde. Se o problema persistir, entre em contacto com o meu desenvolvedor.
no-scrobbled-yet = 
    <b>Parece que você ainda não fez scrobble de nenhuma música no Last.fm.</b>

    Se você estiver enfrentando problemas com o Last.fm, visite last.fm/about/trackmymusic para saber como conectar sua conta ao seu aplicativo de música.
lastfm-playing = 
   <b><a href='https://last.fm/user/{ $lastFMUsername }'>{ $firstName }</a></b> { $nowplaying ->
       [true] está ouvindo
      *[false] estava ouvindo
   } pela <b>{ $playcount }ª vez</b>:
lastfm-help =
    <b>Last.FM Scobbles</b>

    <b>Scrobble</b> é um recurso do Last.fm que registra automaticamente as músicas que você está ouvindo ou ouviu para o serviço.
    <b>Para saber mais, <a href='https://www.last.fm/pt/about/trackmymusic'>clique aqui</a>.</b>

    <b>— Comandos:</b>
    <b>/setuser (nome de usuário):</b> Define seu nome de usuário do Last.fm.
    <b>/lastfm | /lp:</b> Exibe a música que você está ouvindo ou ouviu recentemente.
    <b>/album | /alb:</b> Exibe o álbum que você está ouvindo ou ouviu recentemente.
    <b>/artist   | /art:</b> Exibe o artista que você está ouvindo ou ouviu recentemente.