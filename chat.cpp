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
#include <algorithm>
#include <set>
#include <unistd.h>

using namespace std;
using json = nlohmann::json;

const string VERSION = "5.r";
const string SB_URL = "https://ilszhdmqxsoixcefeoqa.supabase.co/rest/v1/messages";
const string SB_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc";
const int PUA_START = 0xE000;

string my_pass, my_nick, my_room, cfg;
WINDOW *chat_win, *input_win;
mutex mtx;

vector<pair<string, string>> chat_history;
set<string> known_ids; 
int scroll_pos = 0;
const int LOAD_STEP = 15;
bool need_redraw = true;
bool is_loading = false;

// --- УВЕДОМЛЕНИЯ С ПЕРЕХОДОМ В ЧАТ ---
void notify(string author, string text) {
    if (author == my_nick || author.empty()) return;
    string clean_text = text;
    replace(clean_text.begin(), clean_text.end(), '\'', ' ');
    replace(clean_text.begin(), clean_text.end(), '\"', ' ');
    
    // При клике на уведомление открывается ярлык ~/.shortcuts/chat
    string cmd = "termux-notification --title 'Чат: " + author + "' --content '" + clean_text + 
                 "' --id fntm_notif --priority high --sound " +
                 "--action 'termux-open-url termux://shortcuts/chat'";
    system(cmd.c_str());
}

// --- КРИПТО ---
string aes_256(string text, string pass, bool enc) {
    unsigned char key[32], iv[16] = {0};
    SHA256((unsigned char*)pass.c_str(), pass.length(), key);
    EVP_CIPHER_CTX *ctx = EVP_CIPHER_CTX_new();
    int len, flen; unsigned char out[8192];
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

// --- ВСПОМОГАТЕЛЬНЫЕ ---
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
    ((string*)up)->append((char*)ptr, size * nmemb);
    return size * nmemb;
}

string request(string method, int limit, int offset, string body = "") {
    CURL* curl = curl_easy_init();
    string resp;
    if(curl) {
        struct curl_slist* h = NULL;
        h = curl_slist_append(h, ("apikey: " + SB_KEY).c_str());
        h = curl_slist_append(h, ("Authorization: Bearer " + SB_KEY).c_str());
        h = curl_slist_append(h, "Content-Type: application/json");
        string url = SB_URL + "?chat_key=eq." + my_room + "&order=id.desc&limit=" + to_string(limit) + "&offset=" + to_string(offset);
        if (method == "POST") { curl_easy_setopt(curl, CURLOPT_POST, 1L); curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str()); }
        curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
        curl_easy_setopt(curl, CURLOPT_HTTPHEADER, h);
        curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_cb);
        curl_easy_setopt(curl, CURLOPT_WRITEDATA, &resp);
        curl_easy_perform(curl); curl_easy_cleanup(curl);
    }
    return resp;
}

// --- ВОРКЕР С ЗАЩИТОЙ ОТ ПАДЕНИЙ ---
void background_worker() {
    bool is_first_run = true;
    long long max_seen_id = 0;
    while(true) {
        try {
            string res = request("GET", 10, 0);
            if (!res.empty() && res[0] == '[') {
                auto data = json::parse(res);
                if (is_first_run) {
                    for (auto& item : data) {
                        long long id = item.value("id", 0LL);
                        known_ids.insert(to_string(id));
                        if (id > max_seen_id) max_seen_id = id;
                    }
                    is_first_run = false;
                } else {
                    for (int i = data.size()-1; i >= 0; i--) {
                        long long cur_id = data[i].value("id", 0LL);
                        string s_id = to_string(cur_id);
                        if (known_ids.find(s_id) == known_ids.end()) {
                            if (cur_id > max_seen_id) {
                                string snd = data[i].value("sender", "");
                                string dec = aes_256(from_z(data[i].value("payload", "")), my_pass, false);
                                if (dec != "ERR") notify(snd, dec);
                                max_seen_id = cur_id;
                            }
                            known_ids.insert(s_id);
                        }
                    }
                }
            }
        } catch (...) { /* Пропуск ошибок JSON/сети */ }
        sleep(10);
    }
}

// --- ИНТЕРФЕЙС ---
void redraw_chat() {
    int my, mx; getmaxyx(chat_win, my, mx);
    werase(chat_win);
    lock_guard<mutex> l(mtx);
    vector<string> wrapped;
    for (const auto& p : chat_history) {
        string msg = p.second;
        string cur = ""; int w = 0;
        for (size_t i = 0; i < msg.length(); ) {
            int len = 1; unsigned char c = (unsigned char)msg[i];
            if (c >= 0xf0) len = 4; else if (c >= 0xe0) len = 3; else if (c >= 0xc0) len = 2;
            if (w + 1 > mx) { wrapped.push_back(cur); cur = ""; w = 0; }
            cur += msg.substr(i, len); w++; i += len;
        }
        if (!cur.empty()) wrapped.push_back(cur);
    }
    int total = wrapped.size();
    if (total > 0) {
        if (scroll_pos >= total) scroll_pos = total - 1;
        int end = total - 1 - scroll_pos;
        int start = max(0, end - (my - 1));
        for (int i = start, row = 0; i <= end && i < total; i++) mvwaddstr(chat_win, row++, 0, wrapped[i].c_str());
    }
    wrefresh(chat_win);
}

void load_older_messages_async() {
    if (is_loading) return;
    is_loading = true;
    thread([](){
        int offset;
        { lock_guard<mutex> l(mtx); offset = chat_history.size(); }
        string r = request("GET", LOAD_STEP, offset);
        if (!r.empty() && r[0] == '[') {
            try {
                auto data = json::parse(r);
                lock_guard<mutex> l(mtx);
                for (auto& item : data) {
                    string id = to_string(item.value("id", 0));
                    if (known_ids.find(id) == known_ids.end()) {
                        string s = item.value("sender", "???"), p = from_z(item.value("payload", ""));
                        string d = aes_256(p, my_pass, false);
                        chat_history.insert(chat_history.begin(), {id, "[" + s + "]: " + d});
                        known_ids.insert(id);
                    }
                }
                need_redraw = true;
            } catch (...) {}
        }
        is_loading = false;
    }).detach();
}

int main(int argc, char** argv) {
    setlocale(LC_ALL, "");
    cfg = string(getenv("HOME")) + "/.fntm/config.dat";

    if (argc > 1 && string(argv[1]) == "--bg") {
        ifstream fi(cfg);
        if(fi) { getline(fi, my_nick); getline(fi, my_pass); getline(fi, my_room); }
        background_worker();
        return 0;
    }

    ifstream fi(cfg);
    if(fi) { getline(fi, my_nick); getline(fi, my_pass); getline(fi, my_room); }

    // --- УМНАЯ ПРОВЕРКА ПРОЦЕССА (БЕЗ ГЛЮКОВ GREP) ---
    char path[1024];
    ssize_t l = readlink("/proc/self/exe", path, sizeof(path)-1);
    if (l != -1) {
        path[l] = '\0';
        string full_path = string(path);
        string filename = full_path.substr(full_path.find_last_of("/\\") + 1);

        // Используем [m]eoww чтобы grep не нашел сам себя
        string smart_grep = "[" + filename.substr(0,1) + "]" + filename.substr(1);
        string check_cmd = "ps aux | grep '" + smart_grep + "' | grep '\\--bg'";
        
        FILE* pipe = popen(check_cmd.c_str(), "r");
        bool already_running = false;
        if (pipe) {
            char buf[128];
            if (fgets(buf, 128, pipe)) already_running = true;
            pclose(pipe);
        }

        if (!already_running) {
            string cmd = full_path + " --bg > /dev/null 2>&1 &";
            system(cmd.c_str());
        }
    }

    initscr(); cbreak(); noecho(); keypad(stdscr, TRUE); curs_set(1);

    if (my_nick.empty()) {
        echo();
        mvprintw(0,0,"Nick: "); char n[32]; getstr(n); my_nick = n;
        mvprintw(1,0,"Pass: "); char p[64]; getstr(p); my_pass = p;
        mvprintw(2,0,"Room: "); char r[64]; getstr(r); my_room = r;
        mkdir((string(getenv("HOME")) + "/.fntm").c_str(), 0700);
        ofstream fo(cfg); fo << my_nick << endl << my_pass << endl << my_room;
        noecho(); clear();
    }

    int my, mx; getmaxyx(stdscr, my, mx);
    chat_win = newwin(my - 5, mx, 0, 0);
    input_win = newwin(5, mx, my - 5, 0);
    keypad(input_win, TRUE); nodelay(input_win, TRUE);

    thread([](){
        while(true) {
            string r = request("GET", 15, 0); 
            if (!r.empty() && r[0] == '[') {
                try {
                    auto data = json::parse(r);
                    bool upd = false;
                    lock_guard<mutex> l(mtx);
                    for (int i = data.size()-1; i >= 0; i--) {
                        string id = to_string(data[i].value("id", 0));
                        if (known_ids.find(id) == known_ids.end()) {
                            string snd = data[i].value("sender", ""), d = aes_256(from_z(data[i].value("payload", "")), my_pass, false);
                            chat_history.push_back({id, "[" + snd + "]: " + d});
                            known_ids.insert(id);
                            upd = true;
                        }
                    }
                    if (scroll_pos == 0) {
                        while (chat_history.size() > 25) {
                            known_ids.erase(chat_history[0].first);
                            chat_history.erase(chat_history.begin());
                            upd = true;
                        }
                    }
                    if (upd) need_redraw = true;
                } catch (...) {}
            }
            this_thread::sleep_for(chrono::seconds(5));
        }
    }).detach();

    string input_buf = "";
    while(true) {
        if (need_redraw) { redraw_chat(); need_redraw = false; }
        werase(input_win); box(input_win, 0, 0);
        mvwprintw(input_win, 1, 1, "[%s@%s] v%s", my_nick.c_str(), my_room.c_str(), VERSION.c_str());
        mvwprintw(input_win, 2, 1, "> %s", input_buf.c_str());
        wrefresh(input_win);

        int ch = wgetch(input_win);
        if (ch == ERR) { usleep(20000); continue; }

        if (ch == KEY_UP) { 
            scroll_pos++; need_redraw = true; 
            if (scroll_pos + 5 > (int)chat_history.size()) load_older_messages_async(); 
        }
        else if (ch == KEY_DOWN) { if (scroll_pos > 0) { scroll_pos--; need_redraw = true; } }
        else if (ch == '\n' || ch == 10 || ch == 13) {
            if (!input_buf.empty()) {
                string m = input_buf; input_buf = "";
                thread([m](){
                    string e = to_z(aes_256(m, my_pass, true));
                    json j = {{"sender", my_nick}, {"payload", e}, {"chat_key", my_room}};
                    request("POST", 0, 0, j.dump());
                }).detach();
                scroll_pos = 0; need_redraw = true;
            } else { endwin(); exit(0); }
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
    return 0;
}
