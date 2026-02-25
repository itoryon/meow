package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
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
	Sender  string `json:"sender"`
	ChatKey string `json:"chat_key"`
	Payload string `json:"payload"`
}

// Функции шифрования вынесены отдельно для чистоты
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
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil || len(ciphertext) < aes.BlockSize {
		return "[Ошибка расшифровки]"
	}
	block, _ := aes.NewCipher(fixedKey)
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return string(ciphertext)
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow.v2")
	window := myApp.NewWindow("Meow Messenger")
	window.Resize(fyne.NewSize(400, 700))

	prefs := myApp.Preferences()
	var currentRoom string
	var currentPass string
	var messageCache []Message

	chatLog := widget.NewMultiLineEntry()
	chatLog.Disable()
	chatScroll := container.NewVScroll(chatLog)
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")
	titleLabel := widget.NewLabel("Выберите чат")

	// Логика обновления сообщений
	go func() {
		for {
			if currentRoom == "" {
				time.Sleep(2 * time.Second)
				continue
			}
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&order=created_at.desc&limit=30", supabaseURL, currentRoom)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				var messagesFromDB []Message
				json.NewDecoder(resp.Body).Decode(&messagesFromDB)
				messageCache = messagesFromDB
				resp.Body.Close()

				var sb strings.Builder
				for i := len(messageCache) - 1; i >= 0; i-- {
					m := messageCache[i]
					decrypted := decrypt(m.Payload, currentPass)
					sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Sender, decrypted))
				}
				chatLog.SetText(sb.String())
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	sidebar := container.NewVBox()
	
	// Обновление списка чатов
	var refreshSidebar func()
	refreshSidebar = func() {
		sidebar.Objects = nil
		sidebar.Add(widget.NewLabelWithStyle("Meow Профиль", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		
		nickEntry := widget.NewEntry()
		nickEntry.SetText(prefs.StringWithFallback("nickname", "Аноним"))
		sidebar.Add(nickEntry)
		sidebar.Add(widget.NewButton("Сохранить ник", func() {
			prefs.SetString("nickname", nickEntry.Text)
		}))
		
		sidebar.Add(widget.NewSeparator())
		sidebar.Add(widget.NewLabel("Мои чаты:"))

		saved := prefs.StringWithFallback("chat_list", "")
		if saved != "" {
			for _, s := range strings.Split(saved, ",") {
				if s == "" { continue }
				parts := strings.Split(s, ":")
				roomName := parts[0]
				passVal := parts[1]
				sidebar.Add(widget.NewButtonWithIcon(theme.MailAttachmentIcon(), roomName, func() {
					currentRoom = roomName
					currentPass = passVal
					titleLabel.SetText("Чат: " + roomName)
					chatLog.SetText("Загрузка...")
				}))
			}
		}
	}

	addChatBtn := widget.NewButtonWithIcon("Добавить чат", theme.ContentAddIcon(), func() {
		rEntry := widget.NewEntry()
		pEntry := widget.NewPasswordEntry()
		items := []*widget.FormItem{
			{Text: "ID комнаты", Widget: rEntry},
			{Text: "Пароль", Widget: pEntry},
		}
		dialog.ShowForm("Новый чат", "ОК", "Отмена", items, func(b bool) {
			if b && rEntry.Text != "" && pEntry.Text != "" {
				old := prefs.StringWithFallback("chat_list", "")
				newEntry := rEntry.Text + ":" + pEntry.Text
				if old == "" {
					prefs.SetString("chat_list", newEntry)
				} else {
					prefs.SetString("chat_list", old+","+newEntry)
				}
				refreshSidebar()
			}
		}, window)
	})

	refreshSidebar()
	
	sendMsg := func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		text := msgInput.Text
		msgInput.SetText("")
		go func() {
			msg := Message{
				Sender:  prefs.StringWithFallback("nickname", "Аноним"),
				ChatKey: currentRoom,
				Payload: encrypt(text, currentPass),
			}
			jsonData, _ := json.Marshal(msg)
			req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(jsonData))
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}

	msgInput.OnSubmitted = func(s string) { sendMsg() }

	// Сборка интерфейса
	topBar := container.NewHBox(titleLabel)
	bottomBar := container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("", theme.MailSendIcon(), sendMsg), msgInput)
	chatContent := container.NewBorder(topBar, bottomBar, nil, nil, chatScroll)
	
	sidebarContainer := container.NewVScroll(container.NewVBox(sidebar, widget.NewSeparator(), addChatBtn))
	split := container.NewHSplit(sidebarContainer, chatContent)
	split.Offset = 0.35

	window.SetContent(split)
	window.ShowAndRun()
}
