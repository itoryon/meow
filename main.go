package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	supabaseURL  = "https://ilszhdmqxsoixcefeoqa.supabase.co"
	supabaseKey  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc"
	avatarSize   = 96
	jpegQuality  = 72
	avatarDpSize = 48
)

type Message struct {
	ID           int    `json:"id"`
	Sender       string `json:"sender"`
	ChatKey      string `json:"chat_key"`
	Payload      string `json:"payload"`
	SenderAvatar string `json:"sender_avatar"`
}

type ChatMessage struct {
	Sender       string
	Text         string
	AvatarBase64 string
}

var (
	avatarCache      = make(map[string]*canvas.Image)
	avatarCacheMutex sync.Mutex

	incomingMessages []Message
	incomingMu       sync.Mutex
)

func fastCrypt(text, key string, decrypt bool) string {
	if len(text) < 16 && decrypt {
		return text
	}
	hKey := make([]byte, 32)
	copy(hKey, key)
	block, _ := aes.NewCipher(hKey)
	if decrypt {
		data, _ := base64.StdEncoding.DecodeString(text)
		if len(data) < 16 {
			return text
		}
		iv, ct := data[:16], data[16:]
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(ct, ct)
		return string(ct)
	}
	ct := make([]byte, 16+len(text))
	iv := ct[:16]
	io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ct[16:], []byte(text))
	return base64.StdEncoding.EncodeToString(ct)
}

func createAvatarThumbnail(r io.Reader) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	if bounds.Dx() <= avatarSize && bounds.Dy() <= avatarSize {
		var buf bytes.Buffer
		jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality})
		return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	}

	ratio := float64(avatarSize) / math.Max(float64(bounds.Dx()), float64(bounds.Dy()))
	newW := int(float64(bounds.Dx()) * ratio)
	newH := int(float64(bounds.Dy()) * ratio)

	resized := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Src, nil)

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func getOrCreateAvatarImage(b64 string) *canvas.Image {
	avatarCacheMutex.Lock()
	defer avatarCacheMutex.Unlock()

	if img, ok := avatarCache[b64]; ok {
		return img
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	cimg := canvas.NewImageFromImage(src)
	cimg.FillMode = canvas.ImageFillContain
	cimg.SetMinSize(fyne.NewSize(avatarDpSize, avatarDpSize))

	avatarCache[b64] = cimg
	return cimg
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow") // ← изменил на ваш app-id
	window := myApp.NewWindow("Imperor")
	window.Resize(fyne.NewSize(450, 800))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	var lastID int

	var chatMessages []ChatMessage
	var chatList *widget.List
	var chatScroll *container.Scroll

	// Получение сообщений (фон)
	go func() {
		for {
			if currentRoom == "" {
				time.Sleep(time.Second)
				continue
			}

			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=50",
				supabaseURL, currentRoom, lastID)

			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == 200 {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()

				if len(msgs) > 0 {
					incomingMu.Lock()
					incomingMessages = append(incomingMessages, msgs...)
					incomingMu.Unlock()
					// просто триггер — обновление будет в тикере
				}
			}

			time.Sleep(2800 * time.Millisecond)
		}
	}()

	// Плавное добавление в UI
	go func() {
		ticker := time.NewTicker(450 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			incomingMu.Lock()
			batch := incomingMessages
			incomingMessages = nil
			incomingMu.Unlock()

			if len(batch) == 0 {
				continue
			}

			myApp.Lifecycle().RunOnMain(func() {  // ← правильный способ в новых Fyne
				added := 0
				for _, m := range batch {
					if m.ID <= lastID {
						continue
					}
					lastID = m.ID

					txt := fastCrypt(m.Payload, currentPass, true)
					chatMessages = append(chatMessages, ChatMessage{
						Sender:       m.Sender,
						Text:         txt,
						AvatarBase64: m.SenderAvatar,
					})
					added++
				}

				if added > 0 {
					chatList.Refresh()

					threshold := float32(120)
					if chatScroll.Offset.Y >= chatScroll.Content.MinSize().Height-chatScroll.Size().Height-threshold {
						chatScroll.ScrollToBottom()
					}
				}
			})
		}
	}()

	// Шаблон одного сообщения
	createItem := func() fyne.CanvasObject {
		defaultAv := canvas.NewCircle(color.RGBA{R: 60, G: 90, B: 180, A: 255})
		avWrap := container.NewGridWrap(fyne.NewSize(avatarDpSize+8, avatarDpSize+8), defaultAv)

		name := canvas.NewText("...", theme.DisabledColor())
		name.TextSize = 13

		text := widget.NewLabel("")
		text.Wrapping = fyne.TextWrapWord

		vbox := container.NewVBox(name, text)
		return container.NewHBox(avWrap, vbox)
	}

	updateItem := func(id widget.ListItemID, obj fyne.CanvasObject) {
		m := chatMessages[id]
		hbox := obj.(*fyne.Container).Objects[0].(*fyne.Container) // HBox
		avWrap := hbox.Objects[0].(*fyne.Container)
		vbox := hbox.Objects[1].(*fyne.Container)

		avObj := canvas.NewCircle(color.RGBA{R: 60, G: 90, B: 180, A: 255})
		if m.AvatarBase64 != "" {
			if cached := getOrCreateAvatarImage(m.AvatarBase64); cached != nil {
				avObj = cached
			}
		}
		avWrap.Objects = []fyne.CanvasObject{avObj}
		avWrap.Refresh()

		vbox.Objects[0].(*canvas.Text).Text = m.Sender
		vbox.Objects[1].(*widget.Label).SetText(m.Text)
	}

	chatList = widget.NewList(
		func() int { return len(chatMessages) },
		createItem,
		updateItem,
	)

	chatScroll = container.NewVScroll(chatList) // ← только вертикальный скролл по умолчанию

	msgInput := widget.NewEntry()
	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" {
			return
		}
		text := msgInput.Text
		msgInput.SetText("")

		go func() {
			m := Message{
				Sender:       prefs.StringWithFallback("nickname", "User"),
				ChatKey:      currentRoom,
				Payload:      fastCrypt(text, currentPass, false),
				SenderAvatar: prefs.String("avatar_data"),
			}
			b, _ := json.Marshal(m)
			req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(b))
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			req.Header.Set("Content-Type", "application/json")
			http.DefaultClient.Do(req)
		}()
	})

	openChat := func(name, pass string) {
		currentRoom, currentPass = name, pass
		chatMessages = nil
		lastID = 0
		chatList.Refresh()

		chatUI := container.NewBorder(
			container.NewHBox(
				widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
					currentRoom = ""
					refreshMainList()
				}),
				widget.NewLabel(name),
			),
			container.NewBorder(nil, nil, nil, sendBtn, msgInput),
			nil, nil,
			chatScroll,
		)

		contentArea.Objects = []fyne.CanvasObject{chatUI}
		contentArea.Refresh()
	}

	showAddChat := func() {
		rIn := widget.NewEntry()
		pIn := widget.NewEntry()
		var d dialog.Dialog
		content := container.NewVBox(
			widget.NewLabel("ID комнаты:"), rIn,
			widget.NewLabel("Ключ:"), pIn,
			widget.NewButton("ВОЙТИ", func() {
				if rIn.Text != "" {
					list := prefs.String("chat_list")
					if !strings.Contains(list, rIn.Text+":") {
						prefs.SetString("chat_list", list+"|"+rIn.Text+":"+pIn.Text)
					}
					d.Hide()
					openChat(rIn.Text, pIn.Text)
				}
			}),
		)
		d = dialog.NewCustom("Новый чат", "X", container.NewPadded(content), window)
		d.Resize(fyne.NewSize(400, 220))
		d.Show()
	}

	var drawer dialog.Dialog
	menuContent := container.NewVBox(
		widget.NewLabelWithStyle("IMPEROR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewButtonWithIcon("Профиль", theme.AccountIcon(), func() {
			drawer.Hide()

			nick := widget.NewEntry()
			nick.SetText(prefs.StringWithFallback("nickname", "User"))

			avatarBtn := widget.NewButton("Изменить аватар", func() {
				dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
					if err != nil || reader == nil {
						return
					}
					defer reader.Close()

					b64, err := createAvatarThumbnail(reader)
					if err != nil {
						dialog.ShowError(err, window)
						return
					}
					prefs.SetString("avatar_data", b64)
					dialog.ShowInformation("Готово", "Аватар обновлён", window)
				}, window)
			})

			var currAv fyne.CanvasObject = widget.NewLabel("Нет аватара")
			if b64 := prefs.String("avatar_data"); b64 != "" {
				if img := getOrCreateAvatarImage(b64); img != nil {
					// создаём копию для показа в профиле (чтобы не менять оригинал в кэше)
					copyImg := canvas.NewImageFromImage(img.Image)
					copyImg.FillMode = canvas.ImageFillContain
					copyImg.SetMinSize(fyne.NewSize(96, 96))
					currAv = copyImg
				}
			}

			cont := container.NewVBox(
				widget.NewLabel("Ник:"), nick,
				widget.NewLabel("Аватар:"), currAv,
				avatarBtn,
			)

			dialog.ShowCustomConfirm("Профиль", "Сохранить", "Отмена", cont,
				func(ok bool) {
					if ok {
						prefs.SetString("nickname", nick.Text)
					}
				}, window)
		}),
		widget.NewButtonWithIcon("Настройки", theme.SettingsIcon(), func() {
			dialog.ShowInformation("Настройки", "В разработке", window)
		}),
	)
	drawer = dialog.NewCustom("Меню", "Закрыть", container.NewPadded(menuContent), window)

	// Определяем здесь, чтобы функции могли их видеть
	var mainList *fyne.Container
	var mainScroll *container.Scroll
	var contentArea *fyne.Container

	refreshMainList = func() {
		mainList.Objects = nil
		saved := strings.Split(prefs.String("chat_list"), "|")
		for _, s := range saved {
			if !strings.Contains(s, ":") {
				continue
			}
			p := strings.Split(s, ":")
			if len(p) < 2 {
				continue
			}
			n, pass := p[0], p[1]
			mainList.Add(widget.NewButtonWithIcon(n, theme.MailComposeIcon(), func() {
				openChat(n, pass)
			}))
		}

		fab := widget.NewButtonWithIcon("", theme.ContentAddIcon(), showAddChat)
		fab.Importance = widget.HighImportance

		hubUI := container.NewBorder(
			container.NewHBox(
				widget.NewButtonWithIcon("", theme.MenuIcon(), func() { drawer.Show() }),
				widget.NewLabel("IMPEROR"),
			),
			nil, nil, nil,
			container.NewStack(
				mainScroll,
				container.NewBorder(nil, nil, nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(fab))),
			),
		)
		contentArea.Objects = []fyne.CanvasObject{hubUI}
		contentArea.Refresh()
	}

	mainList = container.NewVBox()
	mainScroll = container.NewVScroll(mainList)
	contentArea = container.NewStack()

	refreshMainList()
	window.SetContent(contentArea)
	window.ShowAndRun()
}