package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/color"
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
	IsMe   bool
}

var (
	incomingMessages []Message
	incomingMu       sync.Mutex
	chatMessages     []ChatMessage
	chatList         *widget.List
	chatScroll       *container.Scroll
	currentRoom      string
	currentPass      string
	lastID           int
	contentArea      *fyne.Container
	refreshMainList  func()
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

func main() {
	myApp := app.NewWithID("com.itoryon.meow.v50")
	myApp.Settings().SetTheme(theme.DarkTheme())
	window := myApp.NewWindow("Signal Pro")
	window.Resize(fyne.NewSize(450, 800))
	prefs := myApp.Preferences()
	
	contentArea = container.NewStack()

	// Фоновый поток
	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc", supabaseURL, currentRoom, lastID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
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

	// UI ТИкер
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		for range ticker.C {
			incomingMu.Lock()
			if len(incomingMessages) == 0 { incomingMu.Unlock(); continue }
			batch := incomingMessages
			incomingMessages = nil
			incomingMu.Unlock()
			
			nick := prefs.StringWithFallback("nickname", "User")
			for _, m := range batch {
				if m.ID <= lastID { continue }
				lastID = m.ID
				chatMessages = append(chatMessages, ChatMessage{
					Sender: m.Sender,
					Text:   fastCrypt(m.Payload, currentPass, true),
					IsMe:   m.Sender == nick,
				})
			}
			chatList.Refresh()
			chatScroll.ScrollToBottom()
		}
	}()

	chatList = widget.NewList(
		func() int { return len(chatMessages) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Wrapping = fyne.TextWrapWord
			bg := canvas.NewRectangle(color.Transparent)
			bubble := container.NewStack(bg, container.NewPadded(lbl))
			return container.NewHBox(layout.NewSpacer(), bubble, layout.NewSpacer())
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			m := chatMessages[id]
			hbox := obj.(*fyne.Container)
			bubbleStack := hbox.Objects[1].(*fyne.Container)
			bg := bubbleStack.Objects[0].(*canvas.Rectangle)
			lbl := bubbleStack.Objects[1].(*fyne.Container).Objects[0].(*widget.Label)
			lbl.SetText(m.Text)
			if m.IsMe {
				bg.FillColor = color.RGBA{31, 115, 235, 255}
				hbox.Objects[0].Show(); hbox.Objects[2].Hide()
			} else {
				bg.FillColor = color.RGBA{55, 55, 65, 255}
				hbox.Objects[0].Hide(); hbox.Objects[2].Show()
			}
		},
	)
	chatScroll = container.NewVScroll(chatList)

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение")
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
			req.Header.Set("Prefer", "return=minimal")
			(&http.Client{}).Do(req)
		}()
	})

	openChat := func(name, pass string) {
		currentRoom, currentPass = name, pass
		chatMessages = nil; lastID = 0; chatList.Refresh()
		chatUI := container.NewBorder(
			container.NewHBox(widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { currentRoom = ""; refreshMainList() }), widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
			container.NewPadded(container.NewBorder(nil, nil, nil, sendBtn, msgInput)),
			nil, nil, chatScroll,
		)
		contentArea.Objects = []fyne.CanvasObject{chatUI}; contentArea.Refresh()
	}

	refreshMainList = func() {
		mainList := container.NewVBox()
		saved := strings.Split(prefs.String("chat_list"), "|")
		for _, s := range saved {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			name, pass := p[0], p[1]
			btn := widget.NewButtonWithIcon(name, theme.MailComposeIcon(), func() { openChat(name, pass) })
			mainList.Add(container.NewPadded(btn))
		}
		
		fab := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
			id, key := widget.NewEntry(), widget.NewEntry()
			dialog.ShowForm("New Chat", "Join", "Cancel", []*widget.FormItem{{Text: "ID", Widget: id}, {Text: "Key", Widget: key}}, func(ok bool) {
				if ok { prefs.SetString("chat_list", prefs.String("chat_list")+"|"+id.Text+":"+key.Text); openChat(id.Text, key.Text) }
			}, window)
		})

		hubUI := container.NewBorder(
			container.NewHBox(widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
				nick := widget.NewEntry(); nick.SetText(prefs.StringWithFallback("nickname", "User"))
				dialog.ShowCustomConfirm("Settings", "Save", "Close", nick, func(ok bool) { if ok { prefs.SetString("nickname", nick.Text) } }, window)
			}), widget.NewLabelWithStyle("Signal", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
			nil, nil, nil,
			container.NewStack(container.NewVScroll(mainList), container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(fab)), nil, nil)),
		)
		contentArea.Objects = []fyne.CanvasObject{hubUI}; contentArea.Refresh()
	}

	refreshMainList()
	window.SetContent(contentArea)
	window.ShowAndRun()
}
