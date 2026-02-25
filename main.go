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
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)


const (
	supabaseURL = "https://ilszhdmqxsoixcefeoqa.supabase.co"
	supabaseKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc"
)

type Message struct {
	Sender       string `json:"sender"`
	ChatKey      string `json:"chat_key"`
	Payload      string `json:"payload"`
	SenderAvatar string `json:"sender_avatar"`
}

var cachedMenuAvatar fyne.CanvasObject

// --- ФУНКЦИИ ОБРАБОТКИ ---

func compressImage(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil { return "", err }
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 30})
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

// Создание объекта картинки
func getAvatarObj(base64Str string, size float32) fyne.CanvasObject {
	var img *canvas.Image
	if base64Str != "" {
		if idx := strings.Index(base64Str, ","); idx != -1 { base64Str = base64Str[idx+1:] }
		data, err := base64.StdEncoding.DecodeString(base64Str)
		if err == nil { img = canvas.NewImageFromReader(bytes.NewReader(data), "avatar.jpg") }
	}
	if img == nil { img = canvas.NewImageFromResource(theme.AccountIcon()) }
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return img
}

// --- ОКНО ПРОФИЛЯ (TELEGRAM STYLE) ---
func showUserProfile(win fyne.Window, name string, avatarStr string) {
	largeAvatar := getAvatarObj(avatarStr, 250)
	nameLabel := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	nameLabel.Importance = widget.HighImportance

	profileCard := container.NewVBox(
		container.NewCenter(largeAvatar),
		nameLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Пользователь Meow Messenger", fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
	)

	dialog.ShowCustom("Профиль", "Закрыть", profileCard, win)
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow.v8")
	myApp.Settings().SetTheme(theme.DarkTheme())
	window := myApp.NewWindow("Meow Messenger")
	window.Resize(fyne.NewSize(400, 700))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string

	messageBox := container.NewVBox()
	chatScroll := container.NewVScroll(messageBox)
	msgInput := widget.NewEntry()

	cachedMenuAvatar = getAvatarObj(prefs.String("avatar_base64"), 50)

	// --- ЦИКЛ ОБНОВЛЕНИЯ ---
	go func() {
		for {
			if currentRoom == "" { time.Sleep(2 * time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&order=created_at.desc&limit=20", supabaseURL, currentRoom)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err == nil {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()

				messageBox.Objects = nil
				for i := len(msgs) - 1; i >= 0; i-- {
					m := msgs[i]
					txt := decrypt(m.Payload, currentPass)

					// Аватарка-кнопка
					avatarBtn := widget.NewButton("", func() {
						showUserProfile(window, m.Sender, m.SenderAvatar)
					})
					// Накладываем картинку под невидимую кнопку
					avatarView := container.NewStack(getAvatarObj(m.SenderAvatar, 45), avatarBtn)

					row := container.NewHBox(
						avatarView,
						widget.NewRichText(
							&widget.TextSegment{
								Text:  m.Sender + "\n",
								Style: widget.RichTextStyle{Color: color.NRGBA{100, 200, 255, 255}, TextStyle: fyne.TextStyle{Bold: true}},
							},
							&widget.TextSegment{
								Text:  txt,
								Style: widget.RichTextStyle{Color: color.White},
							},
						),
					)
					messageBox.Add(row)
				}
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	// --- ИНТЕРФЕЙС МЕНЮ ---
	sidebar := container.NewVBox()
	sidebarScroll := container.NewVScroll(sidebar)
	chatContent := container.NewBorder(nil, container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text; msgInput.SetText("")
		go func() {
			msg := Message{
				Sender: prefs.StringWithFallback("nickname", "User"),
				ChatKey: currentRoom,
				Payload: encrypt(t, currentPass),
				SenderAvatar: prefs.String("avatar_base64"),
			}
			d, _ := json.Marshal(msg); r, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(d))
			r.Header.Set("apikey", supabaseKey); r.Header.Set("Authorization", "Bearer "+supabaseKey)
			r.Header.Set("Content-Type", "application/json"); (&http.Client{}).Do(r)
		}()
	}), msgInput), nil, nil, chatScroll)

	mainStack := container.NewStack(chatContent)

	var refreshSidebar func()
	refreshSidebar = func() {
		sidebar.Objects = nil
		sidebar.Add(widget.NewLabelWithStyle("МОЙ ПРОФИЛЬ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		
		// Клик по своей аватарке тоже открывает профиль
		myAvaBtn := widget.NewButton("", func() {
			showUserProfile(window, prefs.StringWithFallback("nickname", "User"), prefs.String("avatar_base64"))
		})
		sidebar.Add(container.NewCenter(container.NewStack(cachedMenuAvatar, myAvaBtn)))

		nickEntry := widget.NewEntry(); nickEntry.SetText(prefs.StringWithFallback("nickname", "User"))
		sidebar.Add(nickEntry)
		sidebar.Add(widget.NewButtonWithIcon("Выбрать фото", theme.FileImageIcon(), func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil { return }
				data, _ := io.ReadAll(reader)
				go func() {
					compressed, _ := compressImage(data)
					prefs.SetString("avatar_base64", compressed)
					cachedMenuAvatar = getAvatarObj(compressed, 50)
					refreshSidebar()
				}()
			}, window)
		}))
		sidebar.Add(widget.NewButton("Сохранить ник", func() { prefs.SetString("nickname", nickEntry.Text) }))
		sidebar.Add(widget.NewSeparator())
		
		for _, s := range strings.Split(prefs.StringWithFallback("chat_list", ""), ",") {
			if s == "" { continue }
			p := strings.Split(s, ":"); if len(p) < 2 { continue }
			r, pass := p[0], p[1]
			btn := widget.NewButton(r, func() { currentRoom, currentPass = r, pass; mainStack.Objects = []fyne.CanvasObject{chatContent} })
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				list := strings.Replace(prefs.String("chat_list"), r+":"+pass, "", -1)
				prefs.SetString("chat_list", strings.Trim(list, ",")); refreshSidebar()
			})
			sidebar.Add(container.NewBorder(nil, nil, nil, del, btn))
		}
		sidebar.Add(widget.NewButtonWithIcon("Добавить чат", theme.ContentAddIcon(), func() {
			rid, rps := widget.NewEntry(), widget.NewPasswordEntry()
			dialog.ShowForm("Новый чат", "ОК", "Отмена", []*widget.FormItem{{Text: "ID", Widget: rid}, {Text: "Pass", Widget: rps}}, func(b bool) {
				if b { 
					cl := prefs.String("chat_list")
					if cl == "" { prefs.SetString("chat_list", rid.Text+":"+rps.Text) } else { prefs.SetString("chat_list", cl+","+rid.Text+":"+rps.Text) }
					refreshSidebar() 
				}
			}, window)
		}))
	}

	refreshSidebar()
	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		refreshSidebar()
		split := container.NewHSplit(sidebarScroll, chatContent)
		split.Offset = 0.4; mainStack.Objects = []fyne.CanvasObject{split}
	})

	window.SetContent(container.NewBorder(container.NewHBox(menuBtn, widget.NewLabel("Meow")), nil, nil, nil, mainStack))
	window.ShowAndRun()
}
