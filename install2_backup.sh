#!/data/data/com.termux/files/usr/bin/bash

G='\e[32m'
B='\e[34m'
N='\e[0m'

echo -e "${G}[*] Подготовка системы meoww...${N}"

# Список нужных пакетов
PKGS=("clang" "libcurl" "openssl" "ncurses-utils" "termux-api" "nlohmann-json")

for pkg in "${PKGS[@]}"; do
    if ! dpkg -s "$pkg" >/dev/null 2>&1; then
        echo -e "${B}[!] Установка $pkg...${N}"
        pkg install -y "$pkg"
    fi
done
echo -e "${G}[OK] Установка завершена!${N}"

