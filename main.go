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
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	supabaseURL = "https://ilszhdmqxsoixcefeoqa.supabase.co"
	supabaseKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc"
)

type Message struct {
	ID           int    `json:"id,omitempty"`
	Sender       string `json:"sender"`
	ChatKey      string `json:"chat_key"`
	Payload      string `json:"payload"`
	SenderAvatar string `json:"sender_avatar"`
}

func fastCrypt(text, key string, decrypt bool) string {
	if len(text) < 16 && decrypt { return text }
	hashedKey := make([]byte, 32)
	copy(hashedKey, key)
	block, _ := aes.NewCipher(hashedKey)
	if decrypt {
		data, _ := base64.StdEncoding.DecodeString(text)
		if len(data) < aes.BlockSize { return text }
		iv := data[:aes.BlockSize]
		ciphertext := data[aes.BlockSize:]
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(ciphertext, ciphertext)
		return string(ciphertext)
	}
	ciphertext := make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]
	io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(text))
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func main() {
	myApp := app.NewWithID("com.itoryon.imperor.v35")
	window := myApp.NewWindow("Imperor UI Fix")
	window.Resize(fyne.NewSize(450, 700))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	var lastID int
	
	chatBox := container.NewVBox()
	chatScroll := container.NewVScroll(chatBox)

	viewAvatar := func(pathData string) {
		if !strings.HasPrefix(pathData, "data:image") {
			dialog.ShowInformation("Инфо", "Аватар отсутствует", window)
			return
		}
		pts := strings.Split(pathData, ",")
		raw, _ := base64.StdEncoding.DecodeString(pts[len(pts)-1])
		img, _, _ := image.Decode(bytes.NewReader(raw))
		view := canvas.NewImageFromImage(img)
		view.FillMode = canvas.ImageFillContain
		view.SetMinSize(fyne.NewSize(350, 350))
		dialog.ShowCustom("Профиль", "Закрыть", view, window)
	}

	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=30", supabaseURL, currentRoom, lastID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err == nil && resp.StatusCode == 200 {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()
				for _, m := range msgs {
					lastID = m.ID
					txt := fastCrypt(m.Payload, currentPass, true)
					
					// Создаем КРУЖОЧЕК-аватарку
					circle := canvas.NewCircle(theme.PrimaryColor())
					circle.StrokeWidth = 1
					circle.StrokeColor = color.White

					avData := m.SenderAvatar
					avatarBtn := widget.NewButton("", func() { viewAvatar(avData) })
					avatarBtn.Importance = widget.LowImportance
					
					avatarStack := container.NewStack(circle, canvas.NewText("?", color.White), avatarBtn)
					// Используем GridWrap для задания размера примитивам
					avatarContainer := container.NewGridWrap(fyne.NewSize(32, 32), avatarStack)

					senderName := canvas.NewText(m.Sender, theme.PrimaryColor())
					senderName.TextSize = 10
					
					msgText := widget.NewLabel(txt)
					msgText.Wrapping = fyne.TextWrapWord
					
					row := container.NewHBox(
						container.NewCenter(avatarContainer),
						container.NewVBox(senderName, msgText),
					)
					chatBox.Add(row)
				}
				chatBox.Refresh()
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Написать...")

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text
		msgInput.SetText("")
		go func() {
			m := Message{
				Sender:       prefs.StringWithFallback("nickname", "Meow"),
				ChatKey:      currentRoom,
				Payload:      fastCrypt(t, currentPass, false),
				SenderAvatar: prefs.String("avatar_path"),
			}
			body, _ := json.Marshal(m)
			req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(body))
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			req.Header.Set("Content-Type", "application/json")
			(&http.Client{}).Do(req)
		}()
	})

	bottomBar := container.NewBorder(nil, nil, nil, sendBtn, msgInput)

	window.SetContent(container.NewBorder(
		container.NewHBox(widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
			idI, psI := widget.NewEntry(), widget.NewEntry()
			dialog.ShowForm("Connect", "OK", "X", []*widget.FormItem{
				{Text: "ID", Widget: idI}, {Text: "Key", Widget: psI},
			}, func(ok bool) {
				if ok {
					currentRoom, currentPass = idI.Text, psI.Text
					chatBox.Objects = nil
					lastID = 0
					chatBox.Refresh()
				}
			}, window)
		}), widget.NewLabel("Imperor")),
		container.NewPadded(bottomBar), // Паддинг поможет клавиатуре
		nil, nil,
		chatScroll,
	))

	window.ShowAndRun()
}
