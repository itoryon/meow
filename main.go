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
	ChatKey      string `json:"chat_key"`
	Payload      string `json:"payload"`
	SenderAvatar string `json:"sender_avatar"`
}

func fastCrypt(text, key string, decrypt bool) string {
	if len(text) < 16 && decrypt { return text }
	hKey := make([]byte, 32); copy(hKey, key)
	block, _ := aes.NewCipher(hKey)
	if decrypt {
		data, _ := base64.StdEncoding.DecodeString(text)
		if len(data) < 16 { return text }
		iv, ct := data[:16], data[16:]
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(ct, ct)
		return string(ct)
	}
	ct := make([]byte, 16+len(text))
	iv := ct[:16]; io.ReadFull(rand.Reader, iv)
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ct[16:], []byte(text))
	return base64.StdEncoding.EncodeToString(ct)
}

func main() {
	myApp := app.NewWithID("com.itoryon.imperor.v38")
	window := myApp.NewWindow("Imperor")
	window.Resize(fyne.NewSize(450, 800))

	prefs := myApp.Preferences()
	var currentRoom, currentPass string
	var lastID int
	
	chatBox := container.NewVBox()
	chatScroll := container.NewVScroll(chatBox)

	// Обновление сообщений
	go func() {
		for {
			if currentRoom == "" { time.Sleep(time.Second); continue }
			url := fmt.Sprintf("%s/rest/v1/messages?chat_key=eq.%s&id=gt.%d&order=id.asc&limit=30", supabaseURL, currentRoom, lastID)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err == nil && resp.StatusCode == 200 {
				var msgs []Message
				json.NewDecoder(resp.Body).Decode(&msgs)
				resp.Body.Close()
				for _, m := range msgs {
					lastID = m.ID
					txt := fastCrypt(m.Payload, currentPass, true)
					circle := canvas.NewCircle(theme.PrimaryColor())
					avatar := container.NewGridWrap(fyne.NewSize(36, 36), circle)
					chatBox.Add(container.NewHBox(avatar, container.NewVBox(canvas.NewText(m.Sender, theme.DisabledColor()), widget.NewLabel(txt))))
				}
				chatBox.Refresh(); chatScroll.ScrollToBottom()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	msgInput := widget.NewEntry()
	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if msgInput.Text == "" || currentRoom == "" { return }
		t := msgInput.Text; msgInput.SetText("")
		go func() {
			m := Message{
				Sender: prefs.StringWithFallback("nickname", "User"),
				ChatKey: currentRoom,
				Payload: fastCrypt(t, currentPass, false),
				SenderAvatar: prefs.String("avatar_data"),
			}
			b, _ := json.Marshal(m)
			req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/messages", bytes.NewBuffer(b))
			req.Header.Set("apikey", supabaseKey)
			req.Header.Set("Authorization", "Bearer "+supabaseKey)
			req.Header.Set("Content-Type", "application/json")
			(&http.Client{}).Do(req)
		}()
	})

	// --- ВСПОМОГАТЕЛЬНЫЕ ОКНА ---
	
	// Окно настроек профиля
	showProfile := func() {
		nickIn := widget.NewEntry()
		nickIn.SetText(prefs.String("nickname"))
		d := dialog.NewCustom("Профиль", "Закрыть", container.NewVBox(
			widget.NewLabel("Ник:"), nickIn,
			widget.NewButton("Сохранить", func() { prefs.SetString("nickname", nickIn.Text) }),
		), window)
		d.Resize(fyne.NewSize(400, 700)); d.Show()
	}

	// Полноэкранное добавление чата
	showAddChat := func() {
		roomIn, passIn := widget.NewEntry(), widget.NewEntry()
		var d dialog.Dialog
		content := container.NewVBox(
			widget.NewLabelWithStyle("ДОБАВИТЬ ЧАТ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewLabel("Название:"), roomIn,
			widget.NewLabel("Пароль:"), passIn,
			widget.NewButton("ВОЙТИ И СОХРАНИТЬ", func() {
				if roomIn.Text != "" {
					// Сохраняем в список
					list := prefs.String("chat_list")
					if !strings.Contains(list, roomIn.Text) {
						prefs.SetString("chat_list", list+"|"+roomIn.Text+":"+passIn.Text)
					}
					currentRoom, currentPass = roomIn.Text, passIn.Text
					chatBox.Objects = nil; lastID = 0; chatBox.Refresh()
					d.Hide()
				}
			}),
		)
		d = dialog.NewCustom("Новый чат", "X", container.NewPadded(content), window)
		d.Resize(fyne.NewSize(450, 800)); d.Show()
	}

	// Боковая панель (Drawer)
	var drawer dialog.Dialog
	showDrawer := func() {
		chatListCont := container.NewVBox()
		chats := strings.Split(prefs.String("chat_list"), "|")
		for _, c := range chats {
			if !strings.Contains(c, ":") { continue }
			p := strings.Split(c, ":")
			name, pass := p[0], p[1]
			chatListCont.Add(widget.NewButton(name, func() {
				currentRoom, currentPass = name, pass
				chatBox.Objects = nil; lastID = 0; chatBox.Refresh()
				drawer.Hide()
			}))
		}

		menu := container.NewVBox(
			widget.NewLabelWithStyle("IMPEROR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewButtonWithIcon("Профиль", theme.AccountIcon(), func() { drawer.Hide(); showProfile() }),
			widget.NewButtonWithIcon("Настройки", theme.SettingsIcon(), func() { dialog.ShowInformation("!", "В разработке", window) }),
			widget.NewSeparator(),
			widget.NewLabel("Ваши чаты:"),
			container.NewVScroll(chatListCont),
		)
		drawer = dialog.NewCustom("Меню", "Закрыть", container.NewPadded(menu), window)
		drawer.Resize(fyne.NewSize(300, 800)) // Высота во весь экран
		drawer.Show()
	}

	// Layout
	fab := widget.NewButtonWithIcon("", theme.ContentAddIcon(), showAddChat)
	fab.Importance = widget.HighImportance

	inputArea := container.NewPadded(container.NewBorder(nil, nil, nil, sendBtn, msgInput))
	
	// Контейнер для чата с кнопкой поверх
	chatLayout := container.NewStack(
		chatScroll,
		container.NewBorder(nil, container.NewHBox(container.NewStack(), fab), nil, nil),
	)

	window.SetContent(container.NewBorder(
		container.NewHBox(widget.NewButtonWithIcon("", theme.MenuIcon(), showDrawer), widget.NewLabel("Imperor")),
		inputArea, nil, nil,
		chatLayout,
	))

	window.ShowAndRun()
}
