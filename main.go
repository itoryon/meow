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
	"image/jpeg"
	_ "image/png"
	"io"
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
	avatarDpSize = 42
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
	avatarCache      = make(map[string]fyne.Resource)
	avatarCacheMutex sync.Mutex

	incomingMessages []Message
	incomingMu       sync.Mutex

	chatMessages []ChatMessage
	chatList     *widget.List
	chatScroll   *container.Scroll
	currentRoom  string
	currentPass  string
	lastID       int

	mainList        *fyne.Container
	mainScroll      *container.Scroll
	contentArea     *fyne.Container
	refreshMainList func()
)

func fastCrypt(text, key string, decrypt bool) string {
	if len(text) < 16 && decrypt { return text }
	hKey := make([]byte, 32); copy(hKey, key)
	block, _ := aes.NewCipher(hKey)
	if decrypt {
		data, _ := base64.StdEncoding.DecodeString(text)
		if len(data) < 16 { return text }
		iv, ct := data[:16], data[16:]
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(ct, ct)
		return string(ct)
	}
	ct := make([]byte, 16+len(text))
	iv := ct[:16]; io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ct[16:], []byte(text))
	return base64.StdEncoding.EncodeToString(ct)
}

// Упрощенная обработка аватара для избежания ошибок с draw.Bilinear
func processAvatar(r io.Reader) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil { return "", err }
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 40})
	return base64.StdEncoding.EncodeToString(buf.Bytes()), err
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow")
	window := myApp.NewWindow("Imperor")
	window.Resize(fyne.NewSize(450, 800))
	prefs := myApp.Preferences()

	// Списки
	mainList = container.NewVBox()
	mainScroll = container.NewVScroll(mainList)
	contentArea = container.NewStack()

	// Фоновый поток получения
	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=50", supabaseURL, currentRoom, lastID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err == nil && resp.StatusCode == 200 {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()
				incomingMu.Lock()
				incomingMessages = append(incomingMessages, msgs...)
				incomingMu.Unlock()
			}
			time.Sleep(2500 * time.Millisecond)
		}
	}()

	// UI ТИкер
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		for range ticker.C {
			incomingMu.Lock()
			batch := incomingMessages
			incomingMessages = nil
			incomingMu.Unlock()
			if len(batch) == 0 { continue }
			
			for _, m := range batch {
				if m.ID <= lastID { continue }
				lastID = m.ID
				chatMessages = append(chatMessages, ChatMessage{
					Sender: m.Sender,
					Text: fastCrypt(m.Payload, currentPass, true),
					AvatarBase64: m.SenderAvatar,
				})
			}
			chatList.Refresh()
			chatScroll.ScrollToBottom()
		}
	}()

	chatList = widget.NewList(
		func() int { return len(chatMessages) },
		func() fyne.CanvasObject {
			av := canvas.NewCircle(color.RGBA{R: 60, G: 90, B: 180, A: 255})
			avBox := container.NewStack(container.NewGridWrap(fyne.NewSize(avatarDpSize, avatarDpSize), av))
			name := canvas.NewText("", theme.DisabledColor())
			msg := widget.NewLabel("")
			msg.Wrapping = fyne.TextWrapWord
			return container.NewHBox(avBox, container.NewVBox(name, msg))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			m := chatMessages[id]
			hbox := obj.(*fyne.Container)
			vbox := hbox.Objects[1].(*fyne.Container)
			vbox.Objects[0].(*canvas.Text).Text = m.Sender
			vbox.Objects[1].(*widget.Label).SetText(m.Text)
		},
	)
	chatScroll = container.NewVScroll(chatList)

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")
	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text; msgInput.SetText("")
		go func() {
			m := Message{
				Sender: prefs.StringWithFallback("nickname", "User"),
				ChatKey: currentRoom,
				Payload: fastCrypt(t, currentPass, false),
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
		chatMessages = nil; lastID = 0; chatList.Refresh()
		chatUI := container.NewBorder(
			container.NewHBox(widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
				currentRoom = ""; refreshMainList()
			}), widget.NewLabel(name)),
			container.NewPadded(container.NewBorder(nil, nil, nil, sendBtn, msgInput)),
			nil, nil, chatScroll,
		)
		contentArea.Objects = []fyne.CanvasObject{chatUI}; contentArea.Refresh()
	}

	showAddChat := func() {
		rIn, pIn := widget.NewEntry(), widget.NewEntry()
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
					d.Hide(); openChat(rIn.Text, pIn.Text)
				}
			}),
		)
		d = dialog.NewCustom("Новый чат", "X", container.NewPadded(content), window)
		d.Resize(fyne.NewSize(400, 300)); d.Show()
	}

	var drawer dialog.Dialog
	menu := container.NewVBox(
		widget.NewLabelWithStyle("IMPEROR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewButton("Профиль", func() {
			drawer.Hide()
			nick := widget.NewEntry(); nick.SetText(prefs.StringWithFallback("nickname", "User"))
			avatarBtn := widget.NewButton("Изменить аватар", func() {
				dialog.ShowFileOpen(func(r fyne.URIReadCloser, _ error) {
					if r == nil { return }
					b64, _ := processAvatar(r)
					prefs.SetString("avatar_data", b64)
				}, window)
			})
			dialog.ShowCustomConfirm("Профиль", "OK", "No", container.NewVBox(nick, avatarBtn), func(ok bool) {
				if ok { prefs.SetString("nickname", nick.Text) }
			}, window)
		}),
	)
	drawer = dialog.NewCustom("Меню", "Закрыть", container.NewPadded(menu), window)

	refreshMainList = func() {
		mainList.Objects = nil
		for _, s := range strings.Split(prefs.String("chat_list"), "|") {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			n, ps := p[0], p[1]
			mainList.Add(widget.NewButtonWithIcon(n, theme.MailComposeIcon(), func() { openChat(n, ps) }))
		}
		fab := widget.NewButtonWithIcon("", theme.ContentAddIcon(), showAddChat)
		fab.Importance = widget.HighImportance
		hubUI := container.NewBorder(
			container.NewHBox(widget.NewButtonWithIcon("", theme.MenuIcon(), func() { drawer.Show() }), widget.NewLabel("IMPEROR")),
			nil, nil, nil,
			container.NewStack(mainScroll, container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(fab)), nil, nil)),
		)
		contentArea.Objects = []fyne.CanvasObject{hubUI}; contentArea.Refresh()
	}

	refreshMainList()
	window.SetContent(contentArea)
	window.ShowAndRun()
}
