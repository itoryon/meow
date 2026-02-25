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
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// --- НАСТРОЙКИ SUPABASE ---
const (
	supabaseURL = "https://ilszhdmqxsoixcefeoqa.supabase.co"
	supabaseKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc"
)

// Структура сообщения для таблицы messages
type Message struct {
	CreatedAt string `json:"created_at,omitempty"`
	Sender    string `json:"sender"`
	ChatKey   string `json:"chat_key"`
	Payload   string `json:"payload"`
}

// --- ФУНКЦИИ ШИФРОВАНИЯ ---

func encrypt(text string, key string) string {
	// Подгоняем ключ под 32 байта для AES-256
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

func decrypt(cryptoText string, key string) string {
	fixedKey := make([]byte, 32)
	copy(fixedKey, key)

	ciphertext, _ := base64.StdEncoding.DecodeString(cryptoText)
	block, _ := aes.NewCipher(fixedKey)
	if len(ciphertext) < aes.BlockSize {
		return "[Ошибка расшифровки]"
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return string(ciphertext)
}

// --- ОСНОВНАЯ ЛОГИКА ---

func main() {
	myApp := app.New()
	window := myApp.NewWindow("Meow Messenger")
	window.Resize(fyne.NewSize(400, 600))

	// Поля ввода
	nickInput := widget.NewEntry()
	nickInput.SetPlaceHolder("Твой ник...")
	
	roomInput := widget.NewEntry()
	roomInput.SetPlaceHolder("Комната (chat_key)...")
	
	passInput := widget.NewPasswordEntry()
	passInput.SetPlaceHolder("Пароль (ключ шифрования)...")

	chatLog := widget.NewMultiLineEntry()
	chatLog.Disable() // Только для чтения

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")

	// Функция отправки
	sendMsg := func() {
		if msgInput.Text == "" || nickInput.Text == "" || roomInput.Text == "" {
			return
		}

		encryptedPayload := encrypt(msgInput.Text, passInput.Text)
		
		msg := Message{
			Sender:  nickInput.Text,
			ChatKey: roomInput.Text,
			Payload: encryptedPayload,
		}

		jsonData, _ := json.Marshal(msg)
		
		req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(jsonData))
		req.Header.Set("apikey", supabaseKey)
		req.Header.Set("Authorization", "Bearer "+supabaseKey)
		req.Header.Set("Content-Type", "application/json")

		go func() {
			client := &http.Client{}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				msgInput.SetText("")
			}
		}()
	}

	// Функция обновления чата
	updateChat := func() {
		for {
			if roomInput.Text == "" {
				time.Sleep(2 * time.Second)
				continue
			}

			// Получаем последние 20 сообщений для этой комнаты
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&order=created_at.desc&limit=20", supabaseURL, roomInput.Text)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err == nil {
				var messages []Message
				json.NewDecoder(resp.Body).Decode(&messages)
				resp.Body.Close()

				formattedChat := ""
				// Идем с конца, чтобы новые были внизу
				for i := len(messages) - 1; i >= 0; i-- {
					m := messages[i]
					decrypted := decrypt(m.Payload, passInput.Text)
					formattedChat += fmt.Sprintf("[%s]: %s\n", m.Sender, decrypted)
				}
				chatLog.SetText(formattedChat)
			}
			time.Sleep(3 * time.Second) // Пауза между обновлениями
		}
	}

	msgInput.OnSubmitted = func(s string) { sendMsg() }
	sendBtn := widget.NewButton("Отправить", sendMsg)

	// Разметка
	content := container.NewVBox(
		widget.NewLabel("Настройки входа:"),
		nickInput,
		roomInput,
		passInput,
		widget.NewSeparator(),
		container.NewStack(chatLog), // Обертка для лога
		msgInput,
		sendBtn,
	)

	// Запускаем фоновое обновление
	go updateChat()

	window.SetContent(content)
	window.ShowAndRun()
}
