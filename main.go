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

// --- НАСТРОЙКИ ---
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

var lastMsgID int
var cachedMenuAvatar fyne.CanvasObject
var settingsDialog dialog.Dialog

// Сжатие картинки до экстремально малого размера
func compressImage(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil { return "", err }
	var buf bytes.Buffer
	// Качество 10% - для аватарки в чате больше не нужно, зато летать будет всё
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 10})
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decrypt(cryptoText, key string) string {
	if !strings.Contains(cryptoText, "=") && len(cryptoText) < 16 { return cryptoText }
	fixedKey := make([]byte, 32); copy(fixedKey, key)
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil || len(ciphertext) < aes.BlockSize { return "[Ошибка расшифровки]" }
	block, _ := aes.NewCipher(fixedKey)
	iv := ciphertext[:aes.BlockSize]; ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return string(ciphertext)
}

func encrypt(text, key string) string {
	fixedKey := make([]byte, 32); copy(fixedKey, key)
	block, _ := aes.NewCipher(fixedKey)
	ciphertext := make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]; io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(text))
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func getAvatarObj(base64Str string, size float32) fyne.CanvasObject {
	if base64Str != "" {
		if idx := strings.Index(base64Str, ","); idx != -1 { base64Str = base64Str[idx+1:] }
		data, err := base64.StdEncoding.DecodeString(base64Str)
		if err == nil {
			img := canvas.NewImageFromReader(bytes.NewReader(data), "a.jpg")
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(size, size))
			return img
		}
	}
	ic := canvas.NewImageFromResource(theme.AccountIcon())
	ic.SetMinSize(fyne.NewSize(size, size))
	return ic
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow.v10")
	window := myApp.NewWindow("Meow")
	window.Resize(fyne.NewSize(400, 700))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	messageBox := container.NewVBox()
	chatScroll := container.NewVScroll(messageBox)
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Напишите сообщение...")

	cachedMenuAvatar = getAvatarObj(prefs.String("avatar_base64"), 60)

	// Цикл получения сообщений
	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc", supabaseURL, currentRoom, lastMsgID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err == nil && resp.StatusCode == 200 {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()

				for _, m := range msgs {
					if m.ID > lastMsgID {
						lastMsgID = m.ID
						txt := decrypt(m.Payload, currentPass)
						
						av := getAvatarObj(m.SenderAvatar, 40)
						name := widget.NewLabelWithStyle(m.Sender, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
						msgBody := widget.NewLabel(txt)
						msgBody.Wrapping = fyne.TextWrapBreak
						
						row := container.NewHBox(av, container.NewVBox(name, msgBody))
						messageBox.Add(row)
						chatScroll.ScrollToBottom()
					}
				}
			}
			time.Sleep(2 * time.Second)
		}
	}()

	sidebar := container.NewVBox()
	var refreshSidebar func()
	refreshSidebar = func() {
		sidebar.Objects = nil
		sidebar.Add(widget.NewLabelWithStyle("НАСТРОЙКИ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		sidebar.Add(container.NewCenter(cachedMenuAvatar))
		
		nick := widget.NewEntry()
		nick.SetText(prefs.StringWithFallback("nickname", "User"))
		sidebar.Add(nick)
		
		sidebar.Add(widget.NewButton("Сменить фото", func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if reader == nil { return }
				d, _ := io.ReadAll(reader)
				s, _ := compressImage(d)
				prefs.SetString("avatar_base64", s)
				cachedMenuAvatar = getAvatarObj(s, 60)
				refreshSidebar()
			}, window)
		}))

		sidebar.Add(widget.NewButton("Сохранить ник", func() { prefs.SetString("nickname", nick.Text) }))
		sidebar.Add(widget.NewSeparator())
		
		for _, s := range strings.Split(prefs.StringWithFallback("chat_list", ""), ",") {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			name, pass := p[0], p[1]
			sidebar.Add(widget.NewButton("Войти в: "+name, func() {
				messageBox.Objects = nil
				lastMsgID = 0
				currentRoom, currentPass = name, pass
				if settingsDialog != nil { settingsDialog.Hide() }
			}))
		}

		sidebar.Add(widget.NewButtonWithIcon("Добавить чат", theme.ContentAddIcon(), func() {
			id, ps := widget.NewEntry(), widget.NewPasswordEntry()
			dialog.ShowForm("Новый чат", "ОК", "Отмена", []*widget.FormItem{
				{Text: "ID", Widget: id}, {Text: "Пароль", Widget: ps},
			}, func(b bool) {
				if b {
					old := prefs.String("chat_list")
					prefs.SetString("chat_list", old+","+id.Text+":"+ps.Text)
					refreshSidebar()
				}
			}, window)
		}))
	}

	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		refreshSidebar()
		settingsDialog = dialog.NewCustom("Настройки", "Закрыть", container.NewVScroll(sidebar), window)
		settingsDialog.Resize(fyne.NewSize(350, 500))
		settingsDialog.Show()
	})

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		content := msgInput.Text
		msgInput.SetText("")
		
		go func() {
			m := Message{
				Sender:       prefs.StringWithFallback("nickname", "User"),
				ChatKey:      currentRoom,
				Payload:      encrypt(content, currentPass),
				SenderAvatar: prefs.String("avatar_base64"),
			}
			body, _ := json.Marshal(m)
			req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(body))
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Prefer", "return=minimal")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode >= 400 {
				dialog.ShowError(fmt.Errorf("Ошибка отправки! Проверь RLS в Supabase"), window)
			}
		}()
	})

	window.SetContent(container.NewBorder(
		container.NewHBox(menuBtn, widget.NewLabel("Meow Messenger")),
		container.NewBorder(nil, nil, nil, sendBtn, msgInput),
		nil, nil, chatScroll,
	))
	window.ShowAndRun()
}
