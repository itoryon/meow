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
	"image/jpeg" // Добавлено для сжатия
	_ "image/png"  // Для поддержки чтения PNG
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

// --- НОВАЯ ФУНКЦИЯ СЖАТИЯ ---
func compressImage(data []byte) (string, error) {
	// Декодируем изображение
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	// Создаем буфер для записи сжатого JPEG
	var buf bytes.Buffer
	// Качество 50 — золотая середина между размером и четкостью
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50})
	if err != nil {
		return "", err
	}

	// Кодируем результат в Base64
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/jpeg;base64," + encoded, nil
}

// (Функции encrypt и decrypt остаются без изменений...)
func encrypt(text, key string) string {
	fixedKey := make([]byte, 32); copy(fixedKey, key)
	block, _ := aes.NewCipher(fixedKey)
	ciphertext := make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]; io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(text))
	return base64.StdEncoding.EncodeToString(ciphertext)
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

	// Создание аватарки (как в прошлой версии)
	makeAvatar := func(base64Str string, isLarge bool) fyne.CanvasObject {
		var img *canvas.Image
		if base64Str != "" {
			if strings.HasPrefix(base64Str, "data:image") {
				idx := strings.Index(base64Str, ",")
				if idx != -1 { base64Str = base64Str[idx+1:] }
			}
			data, err := base64.StdEncoding.DecodeString(base64Str)
			if err == nil {
				img = canvas.NewImageFromReader(bytes.NewReader(data), "img.jpg")
			}
		}
		if img == nil { img = canvas.NewImageFromResource(theme.AccountIcon()) }
		
		img.FillMode = canvas.ImageFillContain
		size := fyne.NewSize(45, 45)
		if isLarge { size = fyne.NewSize(350, 350) }
		img.SetMinSize(size)

		return widget.NewButtonWithIcon("", nil, func() {
			if base64Str != "" {
				// Логика просмотра фото в диалоге
				pData, _ := base64.StdEncoding.DecodeString(base64Str)
				fullImg := canvas.NewImageFromReader(bytes.NewReader(pData), "full.jpg")
				fullImg.FillMode = canvas.ImageFillContain
				fullImg.SetMinSize(fyne.NewSize(350, 350))
				dialog.ShowCustom("Просмотр", "ОК", fullImg, window)
			}
		})
	}

	// (Цикл обновления чата остается прежним...)
	go func() {
		for {
			if currentRoom == "" { time.Sleep(2 * time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&order=created_at.desc&limit=15", supabaseURL, currentRoom)
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
					avatar := makeAvatar(m.SenderAvatar, false)
					nameLabel := widget.NewLabelWithStyle(m.Sender, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
					nameLabel.TextColor = color.NRGBA{R: 100, G: 200, B: 255, A: 255}
					contentLabel := widget.NewLabel(txt)
					contentLabel.Wrapping = fyne.TextWrapBreak
					msgRow := container.NewHBox(avatar, container.NewVBox(nameLabel, contentLabel))
					messageBox.Add(msgRow)
				}
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	var refreshSidebar func()
	sidebar := container.NewVBox()
	
	chatContent := container.NewBorder(
		nil,
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
				r.Header.Set("apikey", supabaseKey); r.Header.Set("Authorization", "Bearer "+supabaseKey)
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
		
		avaBtn := makeAvatar(prefs.String("avatar_base64"), false)
		sidebar.Add(container.NewCenter(avaBtn))

		nickEntry := widget.NewEntry()
		nickEntry.SetText(prefs.StringWithFallback("nickname", "User"))
		sidebar.Add(nickEntry)

		// Кнопка выбора фото ТЕПЕРЬ СО СЖАТИЕМ
		sidebar.Add(widget.NewButtonWithIcon("Выбрать и сжать фото", theme.FileImageIcon(), func() {
			fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil { return }
				defer reader.Close()
				
				data, _ := io.ReadAll(reader)
				
				// Показываем процесс "Загрузка..."
				prog := dialog.NewProgressInfinite("Обработка", "Сжимаем изображение...", window)
				prog.Show()

				go func() {
					compressed, err := compressImage(data)
					prog.Hide()
					if err != nil {
						dialog.ShowError(fmt.Errorf("Ошибка сжатия: %v", err), window)
						return
					}
					prefs.SetString("avatar_base64", compressed)
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
		// (Логика списка чатов остается прежней...)
		saved := prefs.StringWithFallback("chat_list", "")
		for _, s := range strings.Split(saved, ",") {
			if s == "" { continue }
			p := strings.Split(s, ":"); if len(p) < 2 { continue }
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
			dialog.ShowForm("ID", "OK", "Cancel", []*widget.FormItem{{Text: "ID", Widget: rid}, {Text: "Pass", Widget: rps}}, func(b bool) {
				if b { prefs.SetString("chat_list", prefs.String("chat_list")+","+rid.Text+":"+rps.Text); refreshSidebar() }
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
