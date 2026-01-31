# chat
Если вы устанавливаете этот "Проект" на android устройство, то вам нужно:
1. Установить termux api, termux widget, termux
2. Скачать файлы: install, install2.sh
3. Выполните команду
cp storage/downloads/install ~ && cp storage/downloads/install2.sh ~ && cd && chmod +x install && chmod +x install2.sh  
6. Запустить файл install2.sh (установит нужные зависимости)
7. Запустить файл install
8. Даллее выполните эту команду:
   
   mkdir -p ~/.shortcuts && echo '#!/data/data/com.termux/files/usr/bin/bash
export TERM=xterm-256color
termux-wake-lock
stty cols 80 rows 24
~/meoww' > ~/.shortcuts/chat && chmod +x ~/.shortcuts/chat

10. После можете сделать termux widget (если не появился chat то перезагрузите виджет.
11. Запускаете из виджета или из ~ директории командой ./meoww
