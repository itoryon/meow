package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// --- КОНСТАНТЫ ---
const (
	SB_URL    = "https://ilszhdmqxsoixcefeoqa.supabase.co/rest/v1/messages"
	SB_KEY    = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc"
	PUA_START = 0xE000
)

var (
	myNick, myPass, myRoom string
	chatLog               *widget.Entry
)

func aes256(text string, pass string, encrypt bool) string {
	key := sha256.Sum256([]byte(pass))
	block, _ := aes.NewCipher(key[:])
	iv := make([]byte, aes.BlockSize) // Нулевой IV

	if encrypt {
		plaintext := []byte(text)
		padding := aes.BlockSize - len(plaintext)%aes.BlockSize
		padtext := bytes.Repeat([]byte{byte(padding)}, padding)
		plaintext = append(plaintext, padtext...)

		ciphertext := make([]byte, len(plaintext))
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(ciphertext, plaintext)
		return string(ciphertext)
	} else {
		ciphertext := []byte(text)
		if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
			return "ERR"
		}
		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(ciphertext, ciphertext)
		padding := int(ciphertext[len(ciphertext)-1])
		if padding > aes.BlockSize { return "ERR" }
		return string(ciphertext[:len(ciphertext)-padding])
	}
}

// --- Z-ШИФРОВАНИЕ (PUA) ---
func toZ(in string) string {
	var res strings.Builder
	for _, b := range []byte(in) {
		res.WriteRune(rune(PUA_START + int(b)))
		res.WriteString("\xCC\xA1")
	}
	return res.String()
}

func fromZ(in string) string {
	var res []byte
	runes := []rune(in)
	for i := 0; i < len(runes); i++ {
		if runes[i] >= rune(PUA_START) && runes[i] <= rune(PUA_START+255) {
			res = append(res, byte(runes[i]-PUA_START))
		}
	}
	return string(res)
}

// --- СЕТЬ ---
func sendMessage(text string) {
	enc := aes256(text, myPass, true)
	payload := toZ(enc)
	msgData := map[string]string{"sender": myNick, "payload": payload, "chat_key": myRoom}
	body, _ := json.Marshal(msgData)

	req, _ := http.NewRequest("POST", SB_URL, bytes.NewBuffer(body))
	req.Header.Set("apikey", SB_KEY)
	req.Header.Set("Authorization", "Bearer "+SB_KEY)
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

// --- GUI ---
func main() {
	myApp := app.New()
	window := myApp.NewWindow("Meow Messenger")
	window.Resize(fyne.NewSize(400, 600))

	// Экраны
	var loginLayout, chatLayout *fyne.Container

	// Поля логина
	nickEntry := widget.NewEntry(); nickEntry.SetPlaceHolder("Ник")
	passEntry := widget.NewPasswordEntry(); passEntry.SetPlaceHolder("Пароль")
	roomEntry := widget.NewEntry(); roomEntry.SetPlaceHolder("Комната")

	// Поля чата
	chatLog = widget.NewMultiLineEntry()
	chatLog.SetReadOnly(true)
	input := widget.NewEntry(); input.SetPlaceHolder("Сообщение...")

	sendBtn := widget.NewButton("Отправить", func() {
		if input.Text != "" {
			go sendMessage(input.Text)
			input.SetText("")
		}
	})

	chatLayout = container.NewBorder(nil, container.NewVBox(input, sendBtn), nil, nil, chatLog)

	loginBtn := widget.NewButton("Войти в чат", func() {
		myNick, myPass, myRoom = nickEntry.Text, passEntry.Text, roomEntry.Text
		if myNick == "" || myPass == "" || myRoom == "" { return }
		
		window.SetContent(chatLayout)

		// Поток получения сообщений
		go func() {
			knownIDs := make(map[float64]bool)
			for {
				url := fmt.Sprintf("%s?chat_key=eq.%s&order=id.asc&limit=20", SB_URL, myRoom)
				req, _ := http.NewRequest("GET", url, nil)
				req.Header.Set("apikey", SB_KEY)
				req.Header.Set("Authorization", "Bearer "+SB_KEY)

				if resp, err := http.DefaultClient.Do(req); err == nil {
					body, _ := ioutil.ReadAll(resp.Body)
					var data []map[string]interface{}
					json.Unmarshal(body, &data)
					for _, m := range data {
						id := m["id"].(float64)
						if !knownIDs[id] {
							sender := m["sender"].(string)
							dec := aes256(fromZ(m["payload"].(string)), myPass, false)
							chatLog.Append(fmt.Sprintf("[%s]: %s\n", sender, dec))
							knownIDs[id] = true
						}
					}
					resp.Body.Close()
				}
				time.Sleep(2 * time.Second)
			}
		}()
	})

	loginLayout = container.NewVBox(widget.NewLabel("MEOW GUI v5.0"), nickEntry, passEntry, roomEntry, loginBtn)
	window.SetContent(loginLayout)
	window.ShowAndRun()
}
