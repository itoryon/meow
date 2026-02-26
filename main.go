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
	"os"
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

var (
	lastMsgID   int
	avatarCache = make(map[string]fyne.CanvasObject)
)

func fastDecrypt(cryptoText, key string) string {
	if len(cryptoText) < 16 { return cryptoText }
	fixedKey := make([]byte, 32); copy(fixedKey, key)
	ciphertext, _ := base64.StdEncoding.DecodeString(cryptoText)
	block, _ := aes.NewCipher(fixedKey)
	iv := ciphertext[:aes.BlockSize]
	stream := cipher.NewCFBDecrypter(block, iv)
	res := ciphertext[aes.BlockSize:]
	stream.XORKeyStream(res, res)
	return string(res)
}

func fastEncrypt(text, key string) string {
	fixedKey := make([]byte, 32); copy(fixedKey, key)
	block, _ := aes.NewCipher(fixedKey)
	ciphertext := make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]; io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(text))
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func getSmallAvatar(base64Str string) fyne.CanvasObject {
	if base64Str == "" {
		ic := canvas.NewImageFromResource(theme.AccountIcon())
		ic.SetMinSize(fyne.NewSize(36, 36))
		return ic
	}
	if obj, ok := avatarCache[base64Str]; ok { return obj }
	parts := strings.Split(base64Str, ",")
	data, _ := base64.StdEncoding.DecodeString(parts[len(parts)-1])
	img := canvas.NewImageFromReader(bytes.NewReader(data), "s.jpg")
	img.SetMinSize(fyne.NewSize(36, 36))
	avatarCache[base64Str] = img
	return img
}

func main() {
	os.Setenv("FYNE_SCALE", "1.1")
	myApp := app.NewWithID("com.itoryon.meow.v19")
	window := myApp.NewWindow("Meow")
	window.Resize(fyne.NewSize(500, 800))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	messageBox := container.NewVBox()
	chatScroll := container.NewVScroll(messageBox)
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Напиши мяу...")

	// ГЛАВНЫЙ ЦИКЛ ОБНОВЛЕНИЯ (ОПТИМИЗИРОВАННЫЙ)
	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=30", supabaseURL, currentRoom, lastMsgID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			
			resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
			if err == nil && resp.StatusCode == 200 {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()
				for _, m := range msgs {
					if m.ID > lastMsgID {
						lastMsgID = m.ID
						txt := fastDecrypt(m.Payload, currentPass)
						av := getSmallAvatar(m.SenderAvatar)
						name := widget.NewLabelWithStyle(m.Sender, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
						body := widget.NewLabel(txt)
						body.Wrapping = fyne.TextWrapWord
						row := container.NewBorder(nil, nil, av, nil, container.NewVBox(name, body))
						messageBox.Add(container.NewPadded(row))
					}
				}
				chatScroll.ScrollToBottom()
			}
			time.Sleep(2 * time.Second)
		}
	}()

	// ДИНАМИЧЕСКИЙ ДИАЛОГ
	var panelDialog dialog.Dialog
	panelHolder := container.NewStack() // Контейнер для смены контента

	// Функция создания Профиля
	var createMainItems func() fyne.CanvasObject
	
	createProfileItems := func() fyne.CanvasObject {
		nick := widget.NewEntry()
		nick.SetText(prefs.String("nickname"))
		nick.SetPlaceHolder("Твой ник...")
		return container.NewVBox(
			widget.NewButtonWithIcon("Назад", theme.NavigateBackIcon(), func() {
				panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
				panelHolder.Refresh()
			}),
			widget.NewSeparator(),
			widget.NewButton("Выбрать фото", func() {
				dialog.ShowFileOpen(func(r fyne.URIReadCloser, _ error) {
					if r == nil { return }
					d, _ := io.ReadAll(r)
					img, _, _ := image.Decode(bytes.NewReader(d))
					var buf bytes.Buffer
					jpeg.Encode(&buf, img, &jpeg.Options{Quality: 15})
					prefs.SetString("avatar_base64", "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(buf.Bytes()))
				}, window)
			}),
			nick,
			widget.NewButton("Сохранить", func() {
				prefs.SetString("nickname", nick.Text)
				dialog.ShowInformation("Meow", "Данные обновлены", window)
			}),
		)
	}

	// Главное меню
	createMainItems = func() fyne.CanvasObject {
		listCont := container.NewVBox(
			widget.NewLabelWithStyle("MEOW MENU", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewButtonWithIcon("Мой Профиль", theme.AccountIcon(), func() {
				panelHolder.Objects = []fyne.CanvasObject{createProfileItems()}
				panelHolder.Refresh()
			}),
			widget.NewSeparator(),
			widget.NewLabel("ЧАТЫ:"),
		)
		
		chats := strings.Split(prefs.StringWithFallback("chat_list", ""), ",")
		for _, s := range chats {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			name, pass := p[0], p[1]
			listCont.Add(widget.NewButton(name, func() {
				messageBox.Objects = nil
				lastMsgID = 0
				currentRoom, currentPass = name, pass
				panelDialog.Hide()
			}))
		}
		return container.NewVScroll(listCont)
	}

	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
		panelDialog = dialog.NewCustom("Панель управления", "Закрыть", panelHolder, window)
		panelDialog.Resize(fyne.NewSize(350, 550))
		panelDialog.Show()
	})

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text; msgInput.SetText("")
		go func() {
			m := Message{
				Sender: prefs.String("nickname"),
				ChatKey: currentRoom,
				Payload: fastEncrypt(t, currentPass),
				SenderAvatar: prefs.String("avatar_base64"),
			}
			b, _ := json.Marshal(m)
			req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(b))
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			req.Header.Set("Content-Type", "application/json")
			(&http.Client{}).Do(req)
		}()
	})

	window.SetContent(container.NewBorder(
		container.NewHBox(menuBtn, widget.NewLabelWithStyle("MEOW", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
		container.NewBorder(nil, nil, nil, sendBtn, msgInput),
		nil, nil, chatScroll,
	))
	window.ShowAndRun()
}
