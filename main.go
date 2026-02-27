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
	supabaseURL = "https://ilszhdmqxsoixcefeoqa.supabase.co"
	supabaseKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc"
)

type Message struct {
	ID           int    `json:"id"`
	Sender       string `json:"sender"`
	ChatKey      string `json:"chat_key"`
	Payload      string `json:"payload"`
	SenderAvatar string `json:"sender_avatar"`
}

type ChatMessage struct {
	Sender string
	Text   string
	Avatar string
}

var (
	incomingMessages []Message
	incomingMu       sync.Mutex
	chatMessages     []ChatMessage
	chatList         *widget.List
	chatScroll       *container.Scroll
	currentRoom, currentPass string
	lastID           int
	contentArea      *fyne.Container
	refreshMainList  func()
)

// Шифрование (AES-CTR)
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

func main() {
	myApp := app.NewWithID("com.itoryon.imperor.v47")
	myApp.Settings().SetTheme(theme.DarkTheme())
	window := myApp.NewWindow("Imperor")
	window.Resize(fyne.NewSize(450, 800))
	prefs := myApp.Preferences()
	contentArea = container.NewStack()

	// Полноэкранный профиль (Signal style)
	showFullProfile := func(name, b64 string) {
		var imgObj fyne.CanvasObject
		if b64 != "" {
			data, _ := base64.StdEncoding.DecodeString(b64)
			rawImg, _, _ := image.Decode(bytes.NewReader(data))
			ci := canvas.NewImageFromImage(rawImg)
			ci.FillMode = canvas.ImageFillContain
			imgObj = ci
		} else {
			imgObj = canvas.NewCircle(color.RGBA{130, 130, 130, 255})
		}

		closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			contentArea.Objects = contentArea.Objects[:len(contentArea.Objects)-1]
			contentArea.Refresh()
		})
		
		profileUI := container.NewMax(
			canvas.NewRectangle(color.NRGBA{0, 0, 0, 245}),
			container.NewBorder(
				container.NewHBox(layout.NewSpacer(), closeBtn), nil, nil, nil,
				container.NewCenter(
					container.NewVBox(
						container.NewGridWrap(fyne.NewSize(300, 300), imgObj),
						widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true, Italic: false}),
						widget.NewLabelWithStyle("Supabase Encrypted", fyne.TextAlignCenter, fyne.TextStyle{}),
					),
				),
			),
		)
		contentArea.Objects = append(contentArea.Objects, profileUI)
		contentArea.Refresh()
	}

	// Поток обновлений (Long polling light)
	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc", supabaseURL, currentRoom, lastID)
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
			time.Sleep(2 * time.Second)
		}
	}()

	// UI рендерер сообщений
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		for range ticker.C {
			incomingMu.Lock()
			if len(incomingMessages) == 0 { incomingMu.Unlock(); continue }
			batch := incomingMessages
			incomingMessages = nil
			incomingMu.Unlock()
			for _, m := range batch {
				if m.ID <= lastID { continue }
				lastID = m.ID
				chatMessages = append(chatMessages, ChatMessage{
					Sender: m.Sender,
					Text:   fastCrypt(m.Payload, currentPass, true),
					Avatar: m.SenderAvatar,
				})
			}
			chatList.Refresh()
			chatScroll.ScrollToBottom()
		}
	}()

	chatList = widget.NewList(
		func() int { return len(chatMessages) },
		func() fyne.CanvasObject {
			avBtn := widget.NewButton("", nil)
			avBtn.Importance = widget.LowImportance
			circle := canvas.NewCircle(color.RGBA{50, 100, 250, 255})
			av := container.NewStack(container.NewGridWrap(fyne.NewSize(42, 42), circle), avBtn)
			
			msgLabel := widget.NewLabel("")
			msgLabel.Wrapping = fyne.TextWrapWord
			bubble := container.NewPadded(msgLabel)
			
			return container.NewHBox(av, container.NewVBox(canvas.NewText("", theme.DisabledColor()), bubble))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			m := chatMessages[id]
			hbox := obj.(*fyne.Container)
			avStack := hbox.Objects[0].(*fyne.Container)
			vbox := hbox.Objects[1].(*fyne.Container)
			
			avStack.Objects[1].(*widget.Button).OnTapped = func() { showFullProfile(m.Sender, m.Avatar) }
			vbox.Objects[0].(*canvas.Text).Text = m.Sender
			vbox.Objects[1].(*fyne.Container).Objects[0].(*widget.Label).SetText(m.Text)
		},
	)
	chatScroll = container.NewVScroll(chatList)

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Signal Message")
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
			}), widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
			container.NewPadded(container.NewBorder(nil, nil, nil, sendBtn, msgInput)),
			nil, nil, chatScroll,
		)
		contentArea.Objects = []fyne.CanvasObject{chatUI}; contentArea.Refresh()
	}

	refreshMainList = func() {
		mainContent := container.NewVBox()
		for _, s := range strings.Split(prefs.String("chat_list"), "|") {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			mainContent.Add(widget.NewButtonWithIcon(p[0], theme.MessageIcon(), func() { openChat(p[0], p[1]) }))
		}
		fab := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
			r, p := widget.NewEntry(), widget.NewEntry()
			dialog.ShowForm("New Chat", "Start", "Cancel", []*widget.FormItem{{Text: "Chat ID", Widget: r}, {Text: "Security Key", Widget: p}}, func(ok bool) {
				if ok { prefs.SetString("chat_list", prefs.String("chat_list")+"|"+r.Text+":"+p.Text); openChat(r.Text, p.Text) }
			}, window)
		})
		hubUI := container.NewBorder(
			container.NewHBox(widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
				// Меню настроек
				nick := widget.NewEntry(); nick.SetText(prefs.StringWithFallback("nickname", "User"))
				dialog.ShowCustomConfirm("Settings", "Save", "Close", container.NewVBox(widget.NewLabel("Nickname"), nick, widget.NewButton("Change Avatar", func() {
					dialog.ShowFileOpen(func(r fyne.URIReadCloser, _ error) {
						if r == nil { return }
						data, _ := io.ReadAll(r); b64 := base64.StdEncoding.EncodeToString(data)
						prefs.SetString("avatar_data", b64)
					}, window)
				})), func(ok bool) { if ok { prefs.SetString("nickname", nick.Text) } }, window)
			}), widget.NewLabelWithStyle("IMPEROR", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
			nil, nil, nil,
			container.NewStack(container.NewVScroll(mainContent), container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(fab)), nil, nil)),
		)
		contentArea.Objects = []fyne.CanvasObject{hubUI}; contentArea.Refresh()
	}

	refreshMainList()
	window.SetContent(contentArea)
	window.ShowAndRun()
}
