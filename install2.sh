#!/data/data/com.termux/files/usr/bin/bash

G='\e[32m'
B='\e[34m'
N='\e[0m'

echo -e "${G}[*] Проверка системных зависимостей...${N}"

PKGS=("clang" "libcurl" "openssl" "termux-api" "nlohmann-json")
MISSING_PKGS=()

# Проверяем, чего не хватает
for pkg in "${PKGS[@]}"; do
    if ! dpkg -s "$pkg" >/dev/null 2>&1; then
        MISSING_PKGS+=("$pkg")
    fi
done

# Если список пуст и httplib.h на месте — выходим
if [ ${#MISSING_PKGS[@]} -eq 0 ] && [ -f "httplib.h" ]; then
    echo -e "${G}[OK] Все зависимости уже установлены. Пропускаю...${N}"
    exit 0
fi

# Если чего-то не хватает — обновляемся и ставим
if [ ${#MISSING_PKGS[@]} -gt 0 ]; then
    echo -e "${B}[!] Устанавливаю недостающее: ${MISSING_PKGS[*]}${N}"
    pkg update -y
    pkg install -y "${MISSING_PKGS[@]}"
fi

# Докачиваем сетевой заголовок, если его нет
if [ ! -f "httplib.h" ]; then
    echo -e "${B}[*] Скачивание httplib.h...${N}"
    curl -L "https://raw.githubusercontent.com/yhirose/cpp-httplib/master/httplib.h" -o "httplib.h"
fi

echo -e "${G}[OK] Подготовка завершена!${N}"
