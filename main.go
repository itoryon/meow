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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Настройки Supabase
const (
	supabaseURL = "https://ilszhdmqxsoixcefeoqa.supabase.co"
	supabaseKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc"
)

type Message struct {
	Sender  string `json:"sender"`
	ChatKey string `json:"chat_key"`
	Payload string `json:"payload"`
}

type ChatSession struct {
	Room string
	Pass string
}

func main() {
	myApp := app.NewWithID("com.itoryon.meow.v2")
	window := myApp.NewWindow("Meow Messenger")
	window.Resize(fyne.NewSize(400, 700))

	prefs := myApp.Preferences()

	// Состояние текущего чата
	var currentRoom string
	var currentPass string
	var messageCache []Message

	// Виджеты интерфейса
	chatList := widget.NewMultiLineEntry()
	chatList.Disable()
	chatScroll := container.NewVScroll(chatList)
	
	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")

	titleLabel := widget.NewLabel("Выберите чат")

	// --- ФУНКЦИИ ШИФРОВАНИЯ ---
	encrypt := func(text, key string) string {
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

	decrypt := func(cryptoText, key string) string {
		fixedKey := make([]byte, 32)
		copy(fixedKey, key)
		ciphertext, _ := base64.StdEncoding.DecodeString(cryptoText)
		if len(ciphertext) < aes.BlockSize { return "[Ошибка]" }
		block, _ := aes.NewCipher(fixedKey)
		iv := ciphertext[:aes.BlockSize]
		ciphertext = ciphertext[aes.BlockSize:]
		stream := cipher.NewCFBDecrypter(block, iv)
		stream.XORKeyStream(ciphertext, ciphertext)
		return string(ciphertext)
	}

	// --- ЛОГИКА ЧАТОВ ---
	loadMessages := func() {
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
				json.NewDecoder(resp.Body).Decode(&messagesFromDB := []Message{})
				messageCache = messagesFromDB
				resp.Body.Close()
			}

			var sb strings.Builder
			for i := len(messageCache) - 1; i >= 0; i-- {
				m := messageCache[i]
				sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Sender, decrypt(m.Payload, currentPass)))
			}
			chatList.SetText(sb.String())
			chatScroll.ScrollToBottom()
			time.Sleep(3 * time.Second)
		}
	}
	go loadMessages()

	// --- МЕНЮ И НАСТРОЙКИ ---
	sidebar := container.NewVBox()
	
	refreshSidebar := func() {
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

		// Список сохраненных чатов (формат "room:pass")
		saved := prefs.StringWithFallback("chat_list", "")
		if saved != "" {
			for _, s := range strings.Split(saved, ",") {
				parts := strings.Split(s, ":")
				roomName := parts[0]
				passVal := parts[1]
				sidebar.Add(widget.NewButton(theme.MailAttachmentIcon(), roomName, func() {
					currentRoom = roomName
					currentPass = passVal
					titleLabel.SetText("Чат: " + roomName)
					messageCache = nil // очистить кэш при смене комнаты
				}))
			}
		}
	}

	// Диалог добавления чата
	addChat := widget.NewButtonWithIcon("Добавить чат", theme.ContentAddIcon(), func() {
		rEntry := widget.NewEntry()
		pEntry := widget.NewPasswordEntry()
		items := []*widget.FormItem{
			{Text: "ID комнаты", Widget: rEntry},
			{Text: "Пароль", Widget: pEntry},
		}
		dialog := widget.NewForm("Новый чат", "ОК", "Отмена", items, func(b bool) {
			if b {
				old := prefs.StringWithFallback("chat_list", "")
				newEntry := rEntry.Text + ":" + pEntry.Text
				if old == "" { prefs.SetString("chat_list", newEntry) } else {
					prefs.SetString("chat_list", old+","+newEntry)
				}
				refreshSidebar()
			}
		}, window)
		dialog.Show()
	})

	refreshSidebar()
	sidebar.Add(widget.NewSeparator())
	sidebar.Add(addChat)

	// Основной лейаут
	topBar := container.NewHBox(
		widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
			// В Fyne Drawer реализуется через скрытие/показ контейнера или смену контента
			// Для мобилок проще сделать модальное окно настроек
		}),
		titleLabel,
	)

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
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
			resp, _ := client.Do(req)
			if resp != nil { resp.Body.Close() }
		}()
	})

	bottomBar := container.NewBorder(nil, nil, nil, sendBtn, msgInput)
	chatContent := container.NewBorder(topBar, bottomBar, nil, nil, chatScroll)
	
	// Используем Split для симуляции бокового меню
	split := container.NewHSplit(sidebar, chatContent)
	split.Offset = 0.3

	window.SetContent(split)
	window.ShowAndRun()
}
