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

// Ультра-легкое AES-CTR шифрование
func fastCrypt(text, key string, decrypt bool) string {
	if len(text) < 16 && decrypt { return text }
	
	// Генерируем ключ 32 байта (AES-256)
	hashedKey := make([]byte, 32)
	copy(hashedKey, key)

	block, _ := aes.NewCipher(hashedKey)
	
	if decrypt {
		data, _ := base64.StdEncoding.DecodeString(text)
		iv := data[:aes.BlockSize]
		ciphertext := data[aes.BlockSize:]
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(ciphertext, ciphertext)
		return string(ciphertext)
	}

	// Шифрование
	ciphertext := make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]
	io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(text))
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func main() {
	myApp := app.NewWithID("com.itoryon.imperor.v33")
	window := myApp.NewWindow("Imperor Secure")
	window.Resize(fyne.NewSize(450, 700))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	var lastID int
	
	chatBox := container.NewVBox()
	chatScroll := container.NewVScroll(chatBox)

	viewAvatar := func(pathData string) {
		if !strings.HasPrefix(pathData, "data:image") {
			dialog.ShowInformation("Инфо", "Нет фото", window)
			return
		}
		pts := strings.Split(pathData, ",")
		raw, _ := base64.StdEncoding.DecodeString(pts[len(pts)-1])
		img, _, _ := image.Decode(bytes.NewReader(raw))
		view := canvas.NewImageFromImage(img)
		view.FillMode = canvas.ImageFillContain
		view.SetMinSize(fyne.NewSize(300, 300))
		dialog.ShowCustom("Аватар", "Закрыть", view, window)
	}

	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=25", supabaseURL, currentRoom, lastID)
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
					// ДЕШИФРОВКА
					txt := fastCrypt(m.Payload, currentPass, true)
					
					avData := m.SenderAvatar
					nameBtn := widget.NewButtonWithIcon(m.Sender, theme.AccountIcon(), func() { viewAvatar(avData) })
					nameBtn.Importance = widget.LowImportance
					
					msgText := widget.NewLabel(txt)
					msgText.Wrapping = fyne.TextWrapWord
					chatBox.Add(container.NewVBox(nameBtn, msgText))
				}
				chatBox.Refresh()
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Безопасное сообщение...")

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text
		msgInput.SetText("")
		go func() {
			m := Message{
				Sender:       prefs.StringWithFallback("nickname", "User"),
				ChatKey:      currentRoom,
				Payload:      fastCrypt(t, currentPass, false), // ШИФРОВАНИЕ
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

	window.SetContent(container.NewBorder(
		container.NewHBox(widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
			idIn, psIn := widget.NewEntry(), widget.NewEntry()
			dialog.ShowForm("Вход", "ОК", "X", []*widget.FormItem{
				{Text: "ID", Widget: idIn}, {Text: "Key", Widget: psIn},
			}, func(ok bool) {
				if ok {
					currentRoom, currentPass = idIn.Text, psIn.Text
					chatBox.Objects = nil
					lastID = 0
					chatBox.Refresh()
				}
			}, window)
		}), widget.NewLabel("Imperor Secure")),
		container.NewBorder(nil, nil, nil, sendBtn, msgInput),
		nil, nil,
		chatScroll,
	))

	window.ShowAndRun()
}
