#!/data/data/com.termux/files/usr/bin/bash

G='\e[32m'
B='\e[34m'
N='\e[0m'

echo -e "${G}[*] Обновление списков пакетов...${N}"
pkg update -y

echo -e "${G}[*] Подготовка системы meoww (Web Edition)...${N}"

# Список нужных пакетов
PKGS=("clang" "libcurl" "openssl" "termux-api" "nlohmann-json")

for pkg in "${PKGS[@]}"; do
    if ! dpkg -s "$pkg" >/dev/null 2>&1; then
        echo -e "${B}[!] Установка $pkg...${N}"
        pkg install -y "$pkg"
    fi
done

# Скачиваем httplib.h, если его нет
if [ ! -f "httplib.h" ]; then
    echo -e "${B}[*] Скачивание сетевого модуля httplib...${N}"
    curl -L "https://raw.githubusercontent.com/yhirose/cpp-httplib/master/httplib.h" -o "httplib.h"
fi

echo -e "${G}[OK] Подготовка завершена! Теперь запустите скрипт установки чата.${N}"
