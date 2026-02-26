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
	myApp := app.NewWithID("com.itoryon.imperor.v36")
	window := myApp.NewWindow("Imperor v36")
	window.Resize(fyne.NewSize(450, 750))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	var lastID int
	
	chatBox := container.NewVBox()
	chatScroll := container.NewVScroll(chatBox)

	// Поток обновлений
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
					circle := canvas.NewCircle(theme.PrimaryColor())
					circle.StrokeWidth = 1; circle.StrokeColor = color.White
					av := m.SenderAvatar
					btn := widget.NewButton("", func() {
						if strings.HasPrefix(av, "data:image") {
							raw, _ := base64.StdEncoding.DecodeString(strings.Split(av, ",")[1])
							img, _, _ := image.Decode(bytes.NewReader(raw))
							dialog.ShowCustom("Аватар", "X", canvas.NewImageFromImage(img), window)
						}
					})
					btn.Importance = widget.LowImportance
					avatar := container.NewGridWrap(fyne.NewSize(36, 36), container.NewStack(circle, btn))
					chatBox.Add(container.NewHBox(avatar, container.NewVBox(canvas.NewText(m.Sender, theme.DisabledColor()), widget.NewLabel(txt))))
				}
				chatBox.Refresh(); chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	msgInput := widget.NewEntry()
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
			(&http.Client{}).Do(req)
		}()
	})

	// --- МЕНЮ НАСТРОЕК ---
	showSettings := func() {
		nickEntry := widget.NewEntry()
		nickEntry.SetText(prefs.String("nickname"))
		
		imgStatus := widget.NewLabel("Фото не выбрано")
		if prefs.String("avatar_data") != "" { imgStatus.SetText("Фото загружено") }

		btnPhoto := widget.NewButton("Выбрать Аватар", func() {
			dialog.ShowFileOpen(func(r fyne.URIReadCloser, _ error) {
				if r == nil { return }
				data, _ := io.ReadAll(r)
				img, _, _ := image.Decode(bytes.NewReader(data))
				var buf bytes.Buffer
				jpeg.Encode(&buf, img, &jpeg.Options{Quality: 20}) // Сжатие для скорости
				prefs.SetString("avatar_data", "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(buf.Bytes()))
				imgStatus.SetText("Готово!")
			}, window)
		})

		roomEntry, passEntry := widget.NewEntry(), widget.NewEntry()
		roomEntry.SetPlaceHolder("Room ID"); passEntry.SetPlaceHolder("Key")

		content := container.NewVBox(
			widget.NewLabelWithStyle("ПРОФИЛЬ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nickEntry, btnPhoto, imgStatus,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("ВХОД В ЧАТ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			roomEntry, passEntry,
			widget.NewButton("ВОЙТИ", func() {
				prefs.SetString("nickname", nickEntry.Text)
				currentRoom, currentPass = roomEntry.Text, passEntry.Text
				chatBox.Objects = nil; lastID = 0; chatBox.Refresh()
			}),
		)
		
		d := dialog.NewCustom("Настройки", "Закрыть", container.NewPadded(content), window)
		d.Resize(fyne.NewSize(400, 500))
		d.Show()
	}

	window.SetContent(container.NewBorder(
		container.NewHBox(widget.NewButtonWithIcon("", theme.SettingsIcon(), showSettings), widget.NewLabel("Imperor")),
		container.NewPadded(container.NewBorder(nil, nil, nil, sendBtn, msgInput)),
		nil, nil,
		chatScroll,
	))
	window.ShowAndRun()
}
