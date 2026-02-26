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
		ic.SetMinSize(fyne.NewSize(34, 34))
		return ic
	}
	if obj, ok := avatarCache[base64Str]; ok { return obj }
	parts := strings.Split(base64Str, ",")
	data, _ := base64.StdEncoding.DecodeString(parts[len(parts)-1])
	img := canvas.NewImageFromReader(bytes.NewReader(data), "s.jpg")
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(34, 34))
	avatarCache[base64Str] = img
	return img
}

func main() {
	// Важные настройки для Android-производительности
	os.Setenv("FYNE_RENDER", "software")
	myApp := app.NewWithID("com.itoryon.imperor.v23")
	window := myApp.NewWindow("Imperor")
	window.Resize(fyne.NewSize(450, 800))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	messageBox := container.NewVBox()
	chatScroll := container.NewVScroll(messageBox)
	
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")

	// Фоновый поток получения данных (более редкий, чтобы не мешать вводу)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			if currentRoom == "" { continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=25", supabaseURL, currentRoom, lastMsgID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			
			resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
			if err == nil && resp.StatusCode == 200 {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()
				if len(msgs) > 0 {
					for _, m := range msgs {
						if m.ID > lastMsgID {
							lastMsgID = m.ID
							txt := fastDecrypt(m.Payload, currentPass)
							av := getSmallAvatar(m.SenderAvatar)
							name := widget.NewLabelWithStyle(m.Sender, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
							body := widget.NewLabel(txt)
							body.Wrapping = fyne.TextWrapWord
							messageBox.Add(container.NewBorder(nil, nil, av, nil, container.NewVBox(name, body)))
						}
					}
					chatScroll.ScrollToBottom()
				}
			}
		}
	}()

	var panelDialog dialog.Dialog
	panelHolder := container.NewStack()

	var createMainItems func() fyne.CanvasObject

	createProfileItems := func() fyne.CanvasObject {
		nick := widget.NewEntry()
		nick.SetText(prefs.String("nickname"))
		preview := canvas.NewImageFromResource(theme.AccountIcon())
		if prefs.String("avatar_base64") != "" {
			pts := strings.Split(prefs.String("avatar_base64"), ",")
			raw, _ := base64.StdEncoding.DecodeString(pts[len(pts)-1])
			preview = canvas.NewImageFromReader(bytes.NewReader(raw), "p.jpg")
		}
		preview.FillMode = canvas.ImageFillContain
		preview.SetMinSize(fyne.NewSize(80, 80))

		return container.NewVBox(
			widget.NewButtonWithIcon("Назад", theme.NavigateBackIcon(), func() {
				panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
				panelHolder.Refresh()
			}),
			container.NewCenter(preview),
			widget.NewButton("Выбрать фото", func() {
				dialog.ShowFileOpen(func(r fyne.URIReadCloser, _ error) {
					if r == nil { return }
					d, _ := io.ReadAll(r)
					img, _, _ := image.Decode(bytes.NewReader(d))
					var buf bytes.Buffer
					jpeg.Encode(&buf, img, &jpeg.Options{Quality: 10})
					prefs.SetString("avatar_base64", "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(buf.Bytes()))
					preview.Image = img
					preview.Refresh()
				}, window)
			}),
			nick,
			widget.NewButton("Сохранить", func() {
				prefs.SetString("nickname", nick.Text)
			}),
		)
	}

	createMainItems = func() fyne.CanvasObject {
		listCont := container.NewVBox(
			widget.NewLabelWithStyle("IMPEROR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewButtonWithIcon("Профиль", theme.AccountIcon(), func() {
				panelHolder.Objects = []fyne.CanvasObject{createProfileItems()}
				panelHolder.Refresh()
			}),
			widget.NewSeparator(),
		)
		
		chatsStr := prefs.StringWithFallback("chat_list", "")
		chats := strings.Split(chatsStr, ",")
		for _, s := range chats {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			name, pass := p[0], p[1]
			
			chatName := name // Local copy
			chatPass := pass

			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				// Удаление конкретного чата
				currentList := strings.Split(prefs.String("chat_list"), ",")
				newList := []string{}
				for _, item := range currentList {
					if item != chatName+":"+chatPass {
						newList = append(newList, item)
					}
				}
				prefs.SetString("chat_list", strings.Join(newList, ","))
				panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
				panelHolder.Refresh()
			})
			
			entryBtn := widget.NewButton("Чат: "+chatName, func() {
				messageBox.Objects = nil
				lastMsgID = 0
				currentRoom, currentPass = chatName, chatPass
				panelDialog.Hide()
			})
			
			listCont.Add(container.NewBorder(nil, nil, nil, delBtn, entryBtn))
		}

		listCont.Add(widget.NewSeparator())
		listCont.Add(widget.NewButtonWithIcon("Добавить", theme.ContentAddIcon(), func() {
			id, ps := widget.NewEntry(), widget.NewPasswordEntry()
			dialog.ShowForm("Новый чат", "OK", "X", []*widget.FormItem{
				{Text: "ID", Widget: id}, {Text: "Key", Widget: ps},
			}, func(b bool) {
				if b { 
					old := prefs.String("chat_list")
					if old != "" { old += "," }
					prefs.SetString("chat_list", old+id.Text+":"+ps.Text)
					panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
					panelHolder.Refresh()
				}
			}, window)
		}))
		return container.NewVScroll(listCont)
	}

	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
		panelDialog = dialog.NewCustom("Настройки", "Закрыть", panelHolder, window)
		panelDialog.Resize(fyne.NewSize(320, 450))
		panelDialog.Show()
	})

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text; msgInput.SetText("")
		go func() {
			m := Message{
				Sender: prefs.StringWithFallback("nickname", "User"),
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

	// СТРОГАЯ ИЗОЛЯЦИЯ НИЖНЕЙ ПАНЕЛИ
	bottomPanel := container.NewPadded(container.NewBorder(nil, nil, nil, sendBtn, msgInput))

	window.SetContent(container.NewBorder(
		container.NewHBox(menuBtn, widget.NewLabelWithStyle("IMPEROR", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})),
		bottomPanel, // Теперь это независимый контейнер
		nil, nil,
		chatScroll,
	))
	window.ShowAndRun()
}
