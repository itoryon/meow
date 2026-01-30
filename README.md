# chat
Если вы устанавливаете этот "Проект" на android устройство, то вам нужно:
1. Установить termux api, termux widget, termux
2. Скачать файлы: install, install2.sh
3. Запустить файл install2.sh (установит нужные зависимости)
4. Запустить файл install
5. Даллее выполните эту команду:
   mkdir -p ~/.shortcuts && echo '#!/data/data/com.termux/files/usr/bin/bash
export TERM=xterm-256color
termux-wake-lock
stty cols 80 rows 24
~/meoww' > ~/.shortcuts/chat && chmod +x ~/.shortcuts/chat
7. После можете сделай termux widget (если не появился meoww то перезагрузите.
8. Запускаете как сказано из виджета или из ~ директории командой ./meoww
