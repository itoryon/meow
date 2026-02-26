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
		ic.SetMinSize(fyne.NewSize(32, 32))
		return ic
	}
	if obj, ok := avatarCache[base64Str]; ok { return obj }
	parts := strings.Split(base64Str, ",")
	data, _ := base64.StdEncoding.DecodeString(parts[len(parts)-1])
	img := canvas.NewImageFromReader(bytes.NewReader(data), "s.jpg")
	img.SetMinSize(fyne.NewSize(32, 32))
	avatarCache[base64Str] = img
	return img
}

func main() {
	// Отключаем лишние анимации Fyne для Android
	os.Setenv("FYNE_ANIMATION", "0")
	myApp := app.NewWithID("com.itoryon.imperor.v24")
	window := myApp.NewWindow("Imperor")
	window.Resize(fyne.NewSize(450, 800))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	
	// Используем более легкий контейнер без динамических расчетов
	messageBox := container.NewWithoutLayout()
	messageBox.Resize(fyne.NewSize(450, 10000)) // Предварительный размер
	
	chatListContainer := container.NewVBox()
	chatScroll := container.NewVScroll(chatListContainer)
	
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")

	// Обновление сообщений
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		for range ticker.C {
			if currentRoom == "" { continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=15", supabaseURL, currentRoom, lastMsgID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			
			resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
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
						
						// Упрощенная структура строки
						row := container.NewHBox(av, container.NewVBox(name, body))
						chatListContainer.Add(row)
					}
				}
				chatScroll.ScrollToBottom()
			}
		}
	}()

	var panelDialog dialog.Dialog
	panelHolder := container.NewStack()

	var createMainItems func() fyne.CanvasObject

	createProfileItems := func() fyne.CanvasObject {
		nick := widget.NewEntry()
		nick.SetText(prefs.String("nickname"))
		return container.NewVBox(
			widget.NewButton("Назад", func() {
				panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
				panelHolder.Refresh()
			}),
			nick,
			widget.NewButton("Сохранить", func() {
				prefs.SetString("nickname", nick.Text)
			}),
		)
	}

	createMainItems = func() fyne.CanvasObject {
		listCont := container.NewVBox(widget.NewLabelWithStyle("IMPEROR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		
		chats := strings.Split(prefs.StringWithFallback("chat_list", ""), ",")
		for _, s := range chats {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			cName, cPass := p[0], p[1]
			
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				current := strings.Split(prefs.String("chat_list"), ",")
				var nl []string
				for _, item := range current {
					if item != cName+":"+cPass { nl = append(nl, item) }
				}
				prefs.SetString("chat_list", strings.Join(nl, ","))
				panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
				panelHolder.Refresh()
			})
			
			entryBtn := widget.NewButton("Чат: "+cName, func() {
				chatListContainer.Objects = nil
				lastMsgID = 0
				currentRoom, currentPass = cName, cPass
				panelDialog.Hide()
			})
			listCont.Add(container.NewBorder(nil, nil, nil, delBtn, entryBtn))
		}

		listCont.Add(widget.NewButton("Добавить чат", func() {
			id, ps := widget.NewEntry(), widget.NewEntry()
			dialog.ShowForm("Добавить", "ОК", "Х", []*widget.FormItem{
				{Text: "ID", Widget: id}, {Text: "Key", Widget: ps},
			}, func(b bool) {
				if b {
					prefs.SetString("chat_list", prefs.String("chat_list")+","+id.Text+":"+ps.Text)
					panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
					panelHolder.Refresh()
				}
			}, window)
		}))
		return container.NewVScroll(listCont)
	}

	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
		panelDialog = dialog.NewCustom("Меню", "Закрыть", panelHolder, window)
		panelDialog.Show()
	})

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text; msgInput.SetText("")
		go func() {
			m := Message{
				Sender: prefs.StringWithFallback("nickname", "User"),
				ChatKey: currentRoom, Payload: fastEncrypt(t, currentPass),
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

	// Использование Split для жесткого разделения ввода и чата
	inputArea := container.NewBorder(nil, nil, nil, sendBtn, msgInput)
	
	window.SetContent(container.NewBorder(
		container.NewHBox(menuBtn, widget.NewLabel("IMPEROR")),
		container.NewPadded(inputArea),
		nil, nil,
		chatScroll,
	))
	window.ShowAndRun()
}
