#include <iostream>
#include <string>
#include <vector>
#include <thread>
#include <mutex>
#include <fstream>
#include <iomanip>
#include <chrono>
#include <sys/stat.h>
#include <ncurses.h>
#include <curl/curl.h>
#include <nlohmann/json.hpp>
#include <openssl/evp.h>
#include <openssl/sha.h>
#include <clocale>

using namespace std;
using json = nlohmann::json;

// --- КОНФИГ И ВЕРСИЯ ---
const string VERSION = "1.0";
const string SB_URL = "https://ilszhdmqxsoixcefeoqa.supabase.co/rest/v1/messages";
const string SB_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc";
const int PUA_START = 0xE000;

string my_pass, my_nick, my_room, cfg;
string last_id = "";
WINDOW *chat_win, *input_win;
mutex mtx;

vector<string> chat_history;
int scroll_pos = 0;
int current_offset = 0;
const int LOAD_STEP = 15;
bool need_redraw = true;

// --- СЕРВИСНЫЕ ФУНКЦИИ ---

void notify(string author, string text) {
    string cmd = "termux-notification --title 'FNTM: " + author + "' --content '" + text + "' --id fntm_ch --sound";
    system(cmd.c_str());
}

string aes_256(string text, string pass, bool enc) {
    unsigned char key[32], iv[16] = {0};
    SHA256((unsigned char*)pass.c_str(), pass.length(), key);
    EVP_CIPHER_CTX *ctx = EVP_CIPHER_CTX_new();
    int len, flen; unsigned char out[4096];
    if(enc) {
        EVP_EncryptInit_ex(ctx, EVP_aes_256_cbc(), NULL, key, iv);
        EVP_EncryptUpdate(ctx, out, &len, (unsigned char*)text.c_str(), text.length());
        EVP_EncryptFinal_ex(ctx, out + len, &flen);
    } else {
        EVP_DecryptInit_ex(ctx, EVP_aes_256_cbc(), NULL, key, iv);
        EVP_DecryptUpdate(ctx, out, &len, (unsigned char*)text.c_str(), text.length());
        if(EVP_DecryptFinal_ex(ctx, out + len, &flen) <= 0) { EVP_CIPHER_CTX_free(ctx); return "ERR"; }
    }
    EVP_CIPHER_CTX_free(ctx);
    return string((char*)out, len + flen);
}

string from_z(string in) {
    string res = "";
    for (size_t i = 0; i < in.length(); ) {
        if ((unsigned char)in[i] == 0xEE) {
            int code = ((in[i+1] & 0x3F) << 6) | (in[i+2] & 0x3F);
            res += (char)(code); i += 3;
        } else if ((unsigned char)in[i] == 0xCC) { i += 2; } else { i++; }
    }
    return res;
}

string to_z(string in) {
    string res = "";
    for (unsigned char b : in) {
        int code = PUA_START + b;
        res += (char)(0xEE); res += (char)(0x80 | ((code >> 6) & 0x3F)); res += (char)(0x80 | (code & 0x3F));
        res += "\xCC\xA1";
    }
    return res;
}

size_t write_cb(void* ptr, size_t size, size_t nmemb, void* up) {
    ((string*)up)->append((char*)ptr, size * nmemb); return size * nmemb;
}

string request(string method, int limit, int offset, string body = "") {
    CURL* curl = curl_easy_init();
    string resp;
    if(curl) {
        struct curl_slist* h = NULL;
        h = curl_slist_append(h, ("apikey: " + SB_KEY).c_str());
        h = curl_slist_append(h, ("Authorization: Bearer " + SB_KEY).c_str());
        h = curl_slist_append(h, "Content-Type: application/json");
        string url = SB_URL;
        if (method == "GET") {
            url += "?chat_key=eq." + my_room + "&order=created_at.desc";
            url += "&limit=" + to_string(limit) + "&offset=" + to_string(offset);
        } else {
            curl_easy_setopt(curl, CURLOPT_POST, 1L);
            curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
        }
        curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
        curl_easy_setopt(curl, CURLOPT_HTTPHEADER, h);
        curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
        curl_easy_setopt(curl, CURLOPT_WRITEDATA, &resp);
        curl_easy_setopt(curl, CURLOPT_TIMEOUT, 5L);
        curl_easy_perform(curl); curl_easy_cleanup(curl);
    }
    return resp;
}

// --- ОТРИСОВКА С ПОДДЕРЖКОЙ UTF-8 ---

void redraw_chat() {
    if (!need_redraw) return;
    int max_y, max_x;
    getmaxyx(chat_win, max_y, max_x);
    werase(chat_win);

    lock_guard<mutex> l(mtx);
    vector<string> wrapped_lines;

    for (const auto& msg : chat_history) {
        string current_line = "";
        int current_width = 0;
        for (size_t i = 0; i < msg.length(); ) {
            int char_len = 1;
            unsigned char c = (unsigned char)msg[i];
            if (c >= 0xf0) char_len = 4;
            else if (c >= 0xe0) char_len = 3;
            else if (c >= 0xc0) char_len = 2;

            if (current_width + 1 > max_x) {
                wrapped_lines.push_back(current_line);
                current_line = ""; current_width = 0;
            }
            current_line += msg.substr(i, char_len);
            current_width++; i += char_len;
        }
        if (!current_line.empty()) wrapped_lines.push_back(current_line);
    }

    int total = (int)wrapped_lines.size();
    if (total > 0) {
        int end = total - 1 - scroll_pos;
        int start = end - (max_y - 1);
        if (start < 0) start = 0;
        int row = 0;
        for (int i = start; i <= end && i < total; i++) {
            mvwaddstr(chat_win, row++, 0, wrapped_lines[i].c_str());
        }
    }
    wrefresh(chat_win);
    need_redraw = false;
}

void load_older_messages() {
    current_offset += LOAD_STEP;
    try {
        string r = request("GET", LOAD_STEP, current_offset);
        if (r.empty()) return;
        auto data = json::parse(r);
        lock_guard<mutex> l(mtx);
        for (auto& item : data) {
            string s = item.value("sender", "???"), p = from_z(item.value("payload", ""));
            string d = aes_256(p, my_pass, false);
            chat_history.insert(chat_history.begin(), "[" + s + "]: " + (d == "ERR" ? "[Crypted]" : d));
        }
        need_redraw = true;
    } catch(...) {}
}

void refresh_loop() {
    int sleep_time = 2;
    while(true) {
        try {
            string r = request("GET", 1, 0);
            if (!r.empty()) {
                auto data = json::parse(r);
                if(!data.empty()) {
                    string cid = to_string(data[0].value("id", 0));
                    if (cid != last_id) {
                        string r_full = request("GET", 5, 0);
                        auto d_full = json::parse(r_full);
                        lock_guard<mutex> l(mtx);
                        for (int i = d_full.size()-1; i >= 0; i--) {
                            string this_id = to_string(d_full[i].value("id", 0));
                            if (stoll(this_id) > (last_id.empty() ? 0 : stoll(last_id))) {
                                string snd = d_full[i].value("sender", ""), p = from_z(d_full[i].value("payload", ""));
                                string d = aes_256(p, my_pass, false);
                                chat_history.push_back("[" + snd + "]: " + (d == "ERR" ? "[Crypted]" : d));
                                if (snd != my_nick && d != "ERR" && !last_id.empty()) notify(snd, d);
                            }
                        }
                        last_id = cid; need_redraw = true; sleep_time = 2;
                    } else { sleep_time = 4; }
                }
            }
            if (need_redraw && scroll_pos == 0) redraw_chat();
        } catch(...) {}
        this_thread::sleep_for(chrono::seconds(sleep_time));
    }
}

// --- MAIN ---

int main() {
    setlocale(LC_ALL, "");
    cfg = string(getenv("HOME")) + "/.fntm/config.dat";
    ifstream fi(cfg);
    if(fi) { getline(fi, my_nick); getline(fi, my_pass); getline(fi, my_room); }
    initscr(); cbreak(); noecho();
    int my, mx; getmaxyx(stdscr, my, mx);

    if (my_nick.empty()) {
        echo();
        mvprintw(0,0,"Nick: "); refresh(); char n[32]; getstr(n); my_nick = n;
        mvprintw(1,0,"Pass: "); refresh(); char p[64]; getstr(p); my_pass = p;
        mvprintw(2,0,"Room: "); refresh(); char r[64]; getstr(r); my_room = r;
        mkdir((string(getenv("HOME")) + "/.fntm").c_str(), 0700);
        ofstream fo(cfg); fo << my_nick << endl << my_pass << endl << my_room;
        noecho(); clear();
    }

    chat_win = newwin(my - 5, mx, 0, 0);
    input_win = newwin(5, mx, my - 5, 0);
    keypad(input_win, TRUE);
    
    // Загрузка последних сообщений перед стартом
    try {
        auto data = json::parse(request("GET", 15, 0));
        for (int i = data.size()-1; i >= 0; i--) {
            string s = data[i].value("sender", "???"), p = from_z(data[i].value("payload", ""));
            string d = aes_256(p, my_pass, false);
            chat_history.push_back("[" + s + "]: " + (d == "ERR" ? "[Crypted]" : d));
        }
        if(!data.empty()) last_id = to_string(data[0].value("id", 0));
    } catch(...) {}

    thread(refresh_loop).detach();

    string input_buf = "";
    while(true) {
        werase(input_win); box(input_win, 0, 0);
        mvwprintw(input_win, 1, 1, "[%s@%s] v%s", my_nick.c_str(), my_room.c_str(), VERSION.c_str());
        
        string prompt = "> " + input_buf;
        for (size_t i = 0; i < prompt.length(); i += (mx - 2)) {
            if (2 + (int)(i / (mx - 2)) < 4)
                mvwaddstr(input_win, 2 + (i / (mx - 2)), 1, prompt.substr(i, mx - 2).c_str());
        }
        wrefresh(input_win);
        if (need_redraw) redraw_chat();

        int ch = wgetch(input_win);
        if (ch == KEY_UP) { scroll_pos++; need_redraw = true; if (scroll_pos > (int)chat_history.size()) load_older_messages(); }
        else if (ch == KEY_DOWN) { if (scroll_pos > 0) { scroll_pos--; need_redraw = true; } }
        else if (ch == '\n' || ch == 10 || ch == 13) {
            if (input_buf == "/exit") break;
            if (input_buf == "/reset") { remove(cfg.c_str()); break; }
            if (input_buf == "/update") {
                endwin();
                cout << "\e[34m[*] Запуск инсталлера для обновления...\e[0m" << endl;
                system("bash ~/install");
                exit(0);
            }
            if (!input_buf.empty()) {
                string msg = input_buf; input_buf = "";
                thread([msg](){
                    string e = to_z(aes_256(msg, my_pass, true));
                    json j = {{"sender", my_nick}, {"payload", e}, {"chat_key", my_room}};
                    request("POST", 0, 0, j.dump());
                }).detach();
                scroll_pos = 0; need_redraw = true;
            }
        }
        else if (ch == KEY_BACKSPACE || ch == 127 || ch == 8) {
            if (!input_buf.empty()) {
                size_t pos = input_buf.length() - 1;
                while (pos > 0 && (input_buf[pos] & 0xC0) == 0x80) pos--;
                input_buf.erase(pos);
            }
        }
        else if (ch >= 32 && ch < 10000) { input_buf += (char)ch; }
    }
    endwin();
    return 0;
}
