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
	lastMsgID        int
	avatarCache      = make(map[string]fyne.CanvasObject)
	cachedMenuAvatar fyne.CanvasObject
	settingsDialog   dialog.Dialog
)

// Сжатие картинки, чтобы не лагало
func compressImage(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil { return "", err }
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 20})
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func getAvatarCached(base64Str string, size float32) fyne.CanvasObject {
	if base64Str == "" {
		ic := canvas.NewImageFromResource(theme.AccountIcon())
		ic.SetMinSize(fyne.NewSize(size, size))
		return ic
	}
	if obj, ok := avatarCache[base64Str]; ok {
		return obj
	}
	cleanBase := base64Str
	if idx := strings.Index(base64Str, ","); idx != -1 {
		cleanBase = base64Str[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(cleanBase)
	if err == nil {
		img := canvas.NewImageFromReader(bytes.NewReader(data), "a.jpg")
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(size, size))
		avatarCache[base64Str] = img
		return img
	}
	return canvas.NewImageFromResource(theme.AccountIcon())
}

func decrypt(cryptoText, key string) string {
	if !strings.Contains(cryptoText, "=") && len(cryptoText) < 16 { return cryptoText }
	fixedKey := make([]byte, 32); copy(fixedKey, key)
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil || len(ciphertext) < aes.BlockSize { return "[Ошибка]" }
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

func main() {
	os.Setenv("FYNE_SCALE", "1.2")

	myApp := app.NewWithID("com.itoryon.meow.v12")
	window := myApp.NewWindow("Meow Messenger")
	window.Resize(fyne.NewSize(450, 750))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	messageBox := container.NewVBox()
	chatScroll := container.NewVScroll(messageBox)

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Твоё сообщение...")
	
	cachedMenuAvatar = getAvatarCached(prefs.String("avatar_base64"), 60)

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
						av := getAvatarCached(m.SenderAvatar, 44)
						
						name := widget.NewRichText(&widget.TextSegment{
							Text: m.Sender, 
							Style: widget.RichTextStyleStrong,
						})
						msgBody := widget.NewLabel(txt)
						msgBody.Wrapping = fyne.TextWrapBreak
						
						bubble := container.NewVBox(name, msgBody)
						row := container.NewHBox(av, bubble)
						messageBox.Add(container.NewPadded(row))
					}
				}
				chatScroll.ScrollToBottom()
			}
			time.Sleep(2 * time.Second)
		}
	}()

	sidebar := container.NewVBox()
	var refreshSidebar func()
	refreshSidebar = func() {
		sidebar.Objects = nil
		sidebar.Add(widget.NewLabelWithStyle("МЕНЮ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
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
				cachedMenuAvatar = getAvatarCached(s, 60)
				refreshSidebar()
			}, window)
		}))

		sidebar.Add(widget.NewButton("Сохранить ник", func() { prefs.SetString("nickname", nick.Text) }))
		sidebar.Add(widget.NewSeparator())
		
		sidebar.Add(widget.NewLabelWithStyle("МОИ ЧАТЫ", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
		for _, s := range strings.Split(prefs.StringWithFallback("chat_list", ""), ",") {
			if !strings.Contains(s, ":") { continue }
			p := strings.Split(s, ":")
			name, pass := p[0], p[1]
			sidebar.Add(widget.NewButton("Вход: "+name, func() {
				messageBox.Objects = nil
				lastMsgID = 0
				currentRoom, currentPass = name, pass
				if settingsDialog != nil { settingsDialog.Hide() }
			}))
		}
		sidebar.Add(widget.NewButtonWithIcon("Новый чат", theme.ContentAddIcon(), func() {
			id, ps := widget.NewEntry(), widget.NewPasswordEntry()
			dialog.ShowForm("Добавить", "ОК", "Отмена", []*widget.FormItem{
				{Text: "ID", Widget: id}, {Text: "Pass", Widget: ps},
			}, func(b bool) {
				if b && id.Text != "" {
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
		settingsDialog.Resize(fyne.NewSize(380, 500))
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
			(&http.Client{}).Do(req)
		}()
	})

	inputBar := container.NewBorder(nil, nil, nil, sendBtn, container.NewPadded(msgInput))
	topBar := container.NewHBox(menuBtn, widget.NewLabelWithStyle("MEOW", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))

	window.SetContent(container.NewBorder(
		topBar,
		container.NewPadded(inputBar),
		nil, nil,
		chatScroll,
	))
	window.ShowAndRun()
}
