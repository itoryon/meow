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

func main() {
	// Полное отключение всех графических эффектов
	os.Setenv("FYNE_ANIMATION", "0")
	os.Setenv("FYNE_SCALE", "1.0")
	
	myApp := app.NewWithID("com.itoryon.imperor.v26")
	window := myApp.NewWindow("Imperor")
	window.Resize(fyne.NewSize(450, 800))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	
	chatBox := container.NewVBox()
	chatScroll := container.NewVScroll(chatBox)
	
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")

	// Поток обновления (3.5 секунды - баланс между скоростью и лагами)
	go func() {
		for {
			if currentRoom == "" { time.Sleep(2 * time.Second); continue }
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
						
						// Сверхлегкая отрисовка без виджетов
						name := canvas.NewText(m.Sender, theme.PrimaryColor())
						name.TextStyle.Bold = true
						body := widget.NewLabel(txt)
						body.Wrapping = fyne.TextWrapWord
						
						chatBox.Add(container.NewVBox(name, body))
					}
				}
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3500 * time.Millisecond)
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
			widget.NewButton("Фото", func() {
				dialog.ShowFileOpen(func(r fyne.URIReadCloser, _ error) {
					if r == nil { return }
					d, _ := io.ReadAll(r)
					img, _, _ := image.Decode(bytes.NewReader(d))
					var buf bytes.Buffer
					jpeg.Encode(&buf, img, &jpeg.Options{Quality: 10})
					prefs.SetString("avatar_base64", "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(buf.Bytes()))
				}, window)
			}),
			nick,
			widget.NewButton("ОК", func() { prefs.SetString("nickname", nick.Text) }),
		)
	}

	createMainItems = func() fyne.CanvasObject {
		list := container.NewVBox(widget.NewLabel("IMPEROR"))
		list.Add(widget.NewButton("Профиль", func() {
			panelHolder.Objects = []fyne.CanvasObject{createProfileItems()}
			panelHolder.Refresh()
		}))
		
		chats := strings.Split(prefs.StringWithFallback("chat_list", ""), ",")
		for _, s := range chats {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			cName, cPass := p[0], p[1]
			
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				curr := strings.Split(prefs.String("chat_list"), ",")
				var nl []string
				for _, it := range curr { if it != cName+":"+cPass { nl = append(nl, it) } }
				prefs.SetString("chat_list", strings.Join(nl, ","))
				panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
				panelHolder.Refresh()
			})
			
			btn := widget.NewButton("Войти: "+cName, func() {
				chatBox.Objects = nil
				lastMsgID = 0
				currentRoom, currentPass = cName, cPass
				panelDialog.Hide()
			})
			list.Add(container.NewBorder(nil, nil, nil, del, btn))
		}

		list.Add(widget.NewButton("Добавить", func() {
			id, ps := widget.NewEntry(), widget.NewEntry()
			dialog.ShowForm("Add", "OK", "X", []*widget.FormItem{
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
		return container.NewVScroll(list)
	}

	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		panelHolder.Objects = []fyne.CanvasObject{createMainItems()}
		panelDialog = dialog.NewCustom("Menu", "X", panelHolder, window)
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

	// Ультра-плоская структура контента
	window.SetContent(container.NewBorder(
		container.NewHBox(menuBtn, widget.NewLabel("IMPEROR")),
		container.NewBorder(nil, nil, nil, sendBtn, msgInput),
		nil, nil,
		chatScroll,
	))
	window.ShowAndRun()
}
