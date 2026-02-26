package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
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
	ID           int    `json:"id"`
	Sender       string `json:"sender"`
	Payload      string `json:"payload"`
	SenderAvatar string `json:"sender_avatar"` // Теперь тут только PATH или ID
}

func main() {
	// Жесткая деактивация лагов
	os.Setenv("FYNE_RENDER", "software") 
	
	myApp := app.NewWithID("com.itoryon.imperor.v31")
	window := myApp.NewWindow("Imperor v31")
	window.Resize(fyne.NewSize(500, 800))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	var lastID int
	
	chatBox := container.NewVBox()
	chatScroll := container.NewVScroll(chatBox)

	// Функция для открытия аватарки (только когда нужно)
	showAvatar := func(avatarPath string) {
		if avatarPath == "" { return }
		// Здесь логика загрузки картинки по пути из Supabase Storage или кэша
		dialog.ShowInformation("Аватар", "Тут откроется фото: "+avatarPath, window)
	}

	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }

			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc", supabaseURL, currentRoom, lastID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)

			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err != nil { continue }

			var msgs []Message
			json.NewDecoder(resp.Body).Decode(&msgs)
			resp.Body.Close()

			if len(msgs) > 0 {
				for _, m := range msgs {
					lastID = m.ID
					
					// Создаем кликабельное имя (вместо тяжелой картинки)
					path := m.SenderAvatar
					senderBtn := widget.NewButtonWithIcon(m.Sender, theme.AccountIcon(), func() {
						showAvatar(path)
					})
					senderBtn.Importance = widget.LowImportance // Чтобы не выглядело как огромная кнопка

					msgLabel := widget.NewLabel(m.Payload) // Пока без дешифровки для теста скорости
					msgLabel.Wrapping = fyne.TextWrapWord

					chatBox.Add(container.NewVBox(senderBtn, msgLabel))
				}
				chatBox.Refresh()
				chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	msgInput := widget.NewEntry()
	msgInput.SetPlaceHolder("Сообщение...")

	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		
		// При отправке шлем только "ID_AVATAR", а не саму картинку!
		go func() {
			m := Message{
				Sender:       prefs.String("nickname"),
				Payload:      msgInput.Text, // Нужно зашифровать
				SenderAvatar: "user_avatar_777.png", // ПУТЬ, а не base64
			}
			body, _ := json.Marshal(m)
			// POST запрос...
			log.Println("Sent with path:", m.SenderAvatar)
		}()
		msgInput.SetText("")
	})

	window.SetContent(container.NewBorder(
		widget.NewButton("ROOMS", func() {
			// Меню выбора комнат...
		}),
		container.NewBorder(nil, nil, nil, sendBtn, msgInput),
		nil, nil,
		chatScroll,
	))

	window.ShowAndRun()
}
