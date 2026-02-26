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

var lastMsgID int
var cachedMenuAvatar fyne.CanvasObject

func compressImage(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil { return "", err }
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 20})
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decrypt(cryptoText, key string) string {
	fixedKey := make([]byte, 32); copy(fixedKey, key)
	ciphertext, _ := base64.StdEncoding.DecodeString(cryptoText)
	if len(ciphertext) < aes.BlockSize { return "[Зашифровано]" }
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
	var img *canvas.Image
	if base64Str != "" {
		if idx := strings.Index(base64Str, ","); idx != -1 {
			base64Str = base64Str[idx+1:]
		}
		data, err := base64.StdEncoding.DecodeString(base64Str)
		if err == nil {
			img = canvas.NewImageFromReader(bytes.NewReader(data), "a.jpg")
		}
	}
	if img == nil {
		img = canvas.NewImageFromResource(theme.AccountIcon())
	}
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return img
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow.v9")
	myApp.Settings().SetTheme(theme.DarkTheme())
	window := myApp.NewWindow("Meow")
	window.Resize(fyne.NewSize(400, 700))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string

	messageBox := container.NewVBox()
	chatScroll := container.NewVScroll(messageBox)
	msgInput := widget.NewEntry()

	cachedMenuAvatar = getAvatarObj(prefs.String("avatar_base64"), 50)

	// Фон чата
	go func() {
		for {
			if currentRoom == "" {
				time.Sleep(2 * time.Second)
				continue
			}
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc", supabaseURL, currentRoom, lastMsgID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err == nil {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()

				if len(msgs) > 0 {
					for _, m := range msgs {
						txt := decrypt(m.Payload, currentPass)
						if m.ID > lastMsgID { lastMsgID = m.ID }

						avatar := container.NewStack(
							getAvatarObj(m.SenderAvatar, 40),
							widget.NewButton("", func() {
								dialog.ShowCustom("Профиль", "ОК",
									container.NewVBox(container.NewCenter(getAvatarObj(m.SenderAvatar, 200)), 
									widget.NewLabelWithStyle(m.Sender, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})), window)
							}),
						)

						name := widget.NewRichText(&widget.TextSegment{Text: m.Sender, Style: widget.RichTextStyleStrong})
						msgTxt := widget.NewLabel(txt)
						msgTxt.Wrapping = fyne.TextWrapBreak

						row := container.NewHBox(avatar, container.NewVBox(name, msgTxt))
						messageBox.Add(row)
					}
					chatScroll.ScrollToBottom()
				}
			}
			time.Sleep(2 * time.Second)
		}
	}()

	sidebar := container.NewVBox()
	var refreshSidebar func()

	refreshSidebar = func() {
		sidebar.Objects = nil
		sidebar.Add(widget.NewLabelWithStyle("МОЙ ПРОФИЛЬ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		sidebar.Add(container.NewCenter(cachedMenuAvatar))
		
		nickEntry := widget.NewEntry()
		nickEntry.SetText(prefs.StringWithFallback("nickname", "User"))
		sidebar.Add(nickEntry)
		
		sidebar.Add(widget.NewButton("Сменить фото", func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil { return }
				d, _ := io.ReadAll(reader)
				go func() {
					s, _ := compressImage(d)
					prefs.SetString("avatar_base64", s)
					cachedMenuAvatar = getAvatarObj(s, 50)
					refreshSidebar()
				}()
			}, window)
		}))
		
		sidebar.Add(widget.NewButton("Сохранить ник", func() {
			prefs.SetString("nickname", nickEntry.Text)
		}))
		
		sidebar.Add(widget.NewSeparator())
		sidebar.Add(widget.NewLabelWithStyle("МОИ ЧАТЫ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		
		// Список чатов
		chats := strings.Split(prefs.StringWithFallback("chat_list", ""), ",")
		for _, s := range chats {
			if s == "" { continue }
			p := strings.Split(s, ":")
			if len(p) < 2 { continue }
			name, pass := p[0], p[1]
			
			chatBtn := widget.NewButton("Войти в "+name, func() {
				messageBox.Objects = nil
				lastMsgID = 0
				currentRoom, currentPass = name, pass
				dialog.Instance().Hide() // Закрыть настройки при входе
			})
			sidebar.Add(chatBtn)
		}

		// КНОПКА ДОБАВЛЕНИЯ ЧАТА (Вернул!)
		sidebar.Add(widget.NewButtonWithIcon("Добавить новый чат", theme.ContentAddIcon(), func() {
			rid, rps := widget.NewEntry(), widget.NewPasswordEntry()
			dialog.ShowForm("Новый чат", "Добавить", "Отмена", []*widget.FormItem{
				{Text: "ID чата", Widget: rid},
				{Text: "Пароль", Widget: rps},
			}, func(b bool) {
				if b && rid.Text != "" {
					old := prefs.String("chat_list")
					newChat := rid.Text + ":" + rps.Text
					if old == "" { prefs.SetString("chat_list", newChat) } else { prefs.SetString("chat_list", old+","+newChat) }
					refreshSidebar()
				}
			}, window)
		}))
	}

	refreshSidebar()
	
	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		refreshSidebar()
		settingsScroll := container.NewVScroll(sidebar)
		settingsScroll.SetMinSize(fyne.NewSize(350, 500))
		d := dialog.NewCustom("Настройки", "Закрыть", settingsScroll, window)
		d.Resize(fyne.NewSize(380, 550))
		d.Show()
	})

	chatUI := container.NewBorder(
		container.NewHBox(menuBtn, widget.NewLabelWithStyle("Meow Messenger", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
		container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
			if msgInput.Text == "" || currentRoom == "" { return }
			t := msgInput.Text; msgInput.SetText("")
			go func() {
				msg := Message{
					Sender:       prefs.StringWithFallback("nickname", "User"),
					ChatKey:      currentRoom,
					Payload:      encrypt(t, currentPass),
					SenderAvatar: prefs.String("avatar_base64"),
				}
				d, _ := json.Marshal(msg)
				r, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(d))
				r.Header.Set("apikey", supabaseKey)
				r.Header.Set("Authorization", "Bearer "+supabaseKey)
				r.Header.Set("Content-Type", "application/json")
				(&http.Client{}).Do(r)
			}()
		}), msgInput),
		nil, nil, chatScroll,
	)

	window.SetContent(chatUI)
	window.ShowAndRun()
}
