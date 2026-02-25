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

// --- НАСТРОЙКИ SUPABASE ---
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

// --- СЖАТИЕ ФОТО ---
func compressImage(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 40})
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/jpeg;base64," + encoded, nil
}

// --- ШИФРОВАНИЕ ---
func encrypt(text, key string) string {
	fixedKey := make([]byte, 32)
	copy(fixedKey, key)
	block, _ := aes.NewCipher(fixedKey)
	ciphertext := make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]
	io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(text))
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func decrypt(cryptoText, key string) string {
	fixedKey := make([]byte, 32)
	copy(fixedKey, key)
	ciphertext, _ := base64.StdEncoding.DecodeString(cryptoText)
	if len(ciphertext) < aes.BlockSize {
		return "[Зашифровано]"
	}
	block, _ := aes.NewCipher(fixedKey)
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return string(ciphertext)
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow.v6")
	myApp.Settings().SetTheme(theme.DarkTheme())
	window := myApp.NewWindow("Meow Messenger")
	window.Resize(fyne.NewSize(400, 700))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string

	messageBox := container.NewVBox()
	chatScroll := container.NewVScroll(messageBox)
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")

	// Создание аватарки
	makeAvatar := func(base64Str string) fyne.CanvasObject {
		var img *canvas.Image
		if base64Str != "" {
			if strings.HasPrefix(base64Str, "data:image") {
				idx := strings.Index(base64Str, ",")
				if idx != -1 {
					base64Str = base64Str[idx+1:]
				}
			}
			data, err := base64.StdEncoding.DecodeString(base64Str)
			if err == nil {
				img = canvas.NewImageFromReader(bytes.NewReader(data), "img.jpg")
			}
		}
		if img == nil {
			img = canvas.NewImageFromResource(theme.AccountIcon())
		}
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(45, 45))

		// Делаем аватарку кликабельной для просмотра
		btn := widget.NewButton("", func() {
			if base64Str != "" {
				pData, _ := base64.StdEncoding.DecodeString(base64Str)
				fullImg := canvas.NewImageFromReader(bytes.NewReader(pData), "full.jpg")
				fullImg.FillMode = canvas.ImageFillContain
				fullImg.SetMinSize(fyne.NewSize(350, 350))
				dialog.ShowCustom("Просмотр фото", "ОК", fullImg, window)
			}
		})
		return container.NewStack(img, btn)
	}

	// --- ЦИКЛ ОБНОВЛЕНИЯ ЧАТА ---
	go func() {
		for {
			if currentRoom == "" {
				time.Sleep(2 * time.Second)
				continue
			}
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&order=created_at.desc&limit=20", supabaseURL, currentRoom)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
			if err == nil {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()

				messageBox.Objects = nil
				for i := len(msgs) - 1; i >= 0; i-- {
					m := msgs[i]
					txt := decrypt(m.Payload, currentPass)

					// Аватарка
					avatar := makeAvatar(m.SenderAvatar)

					// НИК (Голубой)
					nameLabel := canvas.NewText(m.Sender, color.NRGBA{R: 100, G: 200, B: 255, A: 255})
					nameLabel.TextStyle = fyne.TextStyle{Bold: true}
					nameLabel.TextSize = 12

					// ТЕКСТ СООБЩЕНИЯ (БЕЛЫЙ)
					contentLabel := canvas.NewText(txt, color.White)
					contentLabel.TextSize = 16

					// Упаковка
					msgData := container.NewVBox(nameLabel, contentLabel)
					msgRow := container.NewHBox(avatar, msgData)
					messageBox.Add(msgRow)
				}
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	// --- ИНТЕРФЕЙС ---
	var refreshSidebar func()
	sidebar := container.NewVBox()

	chatContent := container.NewBorder(
		nil,
		container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
			if msgInput.Text == "" || currentRoom == "" {
				return
			}
			t := msgInput.Text
			msgInput.SetText("")
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

	mainContainer := container.NewStack(chatContent)

	refreshSidebar = func() {
		sidebar.Objects = nil
		sidebar.Add(widget.NewLabelWithStyle("Мой Профиль", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))

		// Аватарка в профиле
		sidebar.Add(container.NewCenter(makeAvatar(prefs.String("avatar_base64"))))

		nickEntry := widget.NewEntry()
		nickEntry.SetText(prefs.StringWithFallback("nickname", "User"))
		sidebar.Add(nickEntry)

		sidebar.Add(widget.NewButtonWithIcon("Выбрать фото", theme.FileImageIcon(), func() {
			fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil {
					return
				}
				data, _ := io.ReadAll(reader)
				prog := dialog.NewProgressInfinite("Сжатие", "Обработка фото...", window)
				prog.Show()
				go func() {
					compressed, _ := compressImage(data)
					prefs.SetString("avatar_base64", compressed)
					prog.Hide()
					refreshSidebar()
				}()
			}, window)
			fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
			fd.Show()
		}))

		sidebar.Add(widget.NewButton("Сохранить ник", func() {
			prefs.SetString("nickname", nickEntry.Text)
		}))

		sidebar.Add(widget.NewSeparator())
		sidebar.Add(widget.NewLabel("Чаты:"))

		saved := prefs.StringWithFallback("chat_list", "")
		for _, s := range strings.Split(saved, ",") {
			if s == "" {
				continue
			}
			p := strings.Split(s, ":")
			if len(p) < 2 {
				continue
			}
			r, pass := p[0], p[1]
			row := container.NewBorder(nil, nil, nil,
				widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
					newS := strings.Replace(prefs.String("chat_list"), r+":"+pass, "", -1)
					prefs.SetString("chat_list", strings.Trim(newS, ","))
					refreshSidebar()
				}),
				widget.NewButton(r, func() {
					currentRoom, currentPass = r, pass
					mainContainer.Objects = []fyne.CanvasObject{chatContent}
				}),
			)
			sidebar.Add(row)
		}
		sidebar.Add(widget.NewButtonWithIcon("Добавить чат", theme.ContentAddIcon(), func() {
			rid, rps := widget.NewEntry(), widget.NewPasswordEntry()
			dialog.ShowForm("Новый чат", "ОК", "Отмена", []*widget.FormItem{{Text: "ID", Widget: rid}, {Text: "Pass", Widget: rps}}, func(b bool) {
				if b {
					cl := prefs.String("chat_list")
					if cl == "" {
						prefs.SetString("chat_list", rid.Text+":"+rps.Text)
					} else {
						prefs.SetString("chat_list", cl+","+rid.Text+":"+rps.Text)
					}
					refreshSidebar()
				}
			}, window)
		}))
	}

	refreshSidebar()

	menuBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		refreshSidebar()
		mainContainer.Objects = []fyne.CanvasObject{container.NewHSplit(container.NewVScroll(sidebar), chatContent)}
	})

	window.SetContent(container.NewBorder(container.NewHBox(menuBtn, widget.NewLabel("Meow Messenger")), nil, nil, nil, mainContainer))
	window.ShowAndRun()
}
