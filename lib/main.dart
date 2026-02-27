import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:supabase_flutter/supabase_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:encrypt/encrypt.dart' as enc;

// ─── КОНФИГ ──────────────────────────────────────────────────────────────────
const supabaseUrl = 'https://ilszhdmqxsoixcefeoqa.supabase.co';
const supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc';

// ─── НАСТРОЙКИ ПРИЛОЖЕНИЯ ────────────────────────────────────────────────────
class AppSettings extends ChangeNotifier {
  static final AppSettings _i = AppSettings._();
  factory AppSettings() => _i;
  AppSettings._();

  // Тема
  bool   _darkMode    = true;
  // Акцентный цвет (индекс в списке)
  int    _accentIndex = 0;
  // Размер шрифта
  double _fontSize    = 15;
  // Форма пузырей: 0=rounded, 1=sharp, 2=bubble
  int    _bubbleStyle = 0;
  // Фон чата: 0=plain, 1=dots, 2=lines, 3=grid
  int    _chatBg      = 0;

  bool   get darkMode    => _darkMode;
  int    get accentIndex => _accentIndex;
  double get fontSize    => _fontSize;
  int    get bubbleStyle => _bubbleStyle;
  int    get chatBg      => _chatBg;

  // Список доступных акцентных цветов
  static const accentColors = [
    Color(0xFF6C63FF), // фиолетовый (по умолчанию)
    Color(0xFF2090FF), // синий (как в оригинале)
    Color(0xFF00D4AA), // мятный
    Color(0xFFFF6584), // розовый
    Color(0xFFFF9800), // оранжевый
    Color(0xFF4CAF50), // зелёный
  ];

  Color get accent => accentColors[_accentIndex];

  Future<void> load() async {
    final p = await SharedPreferences.getInstance();
    _darkMode    = p.getBool('darkMode')    ?? true;
    _accentIndex = p.getInt('accentIndex')  ?? 0;
    _fontSize    = p.getDouble('fontSize')  ?? 15;
    _bubbleStyle = p.getInt('bubbleStyle')  ?? 0;
    _chatBg      = p.getInt('chatBg')       ?? 0;
    notifyListeners();
  }

  Future<void> setDarkMode(bool v) async {
    _darkMode = v;
    (await SharedPreferences.getInstance()).setBool('darkMode', v);
    notifyListeners();
  }

  Future<void> setAccent(int v) async {
    _accentIndex = v;
    (await SharedPreferences.getInstance()).setInt('accentIndex', v);
    notifyListeners();
  }

  Future<void> setFontSize(double v) async {
    _fontSize = v;
    (await SharedPreferences.getInstance()).setDouble('fontSize', v);
    notifyListeners();
  }

  Future<void> setBubbleStyle(int v) async {
    _bubbleStyle = v;
    (await SharedPreferences.getInstance()).setInt('bubbleStyle', v);
    notifyListeners();
  }

  Future<void> setChatBg(int v) async {
    _chatBg = v;
    (await SharedPreferences.getInstance()).setInt('chatBg', v);
    notifyListeners();
  }
}

// ─── ШИФРОВАНИЕ — совместимо с itoryon/meow ──────────────────────────────────
String _encryptMsg(String text, String rawKey) {
  if (rawKey.isEmpty) return text;
  final key       = enc.Key.fromUtf8(rawKey.padRight(32).substring(0, 32));
  final iv        = enc.IV.fromLength(16);
  final encrypter = enc.Encrypter(enc.AES(key));
  return encrypter.encrypt(text, iv: iv).base64;
}

String _decryptMsg(String text, String rawKey) {
  if (rawKey.isEmpty) return text;
  try {
    final key       = enc.Key.fromUtf8(rawKey.padRight(32).substring(0, 32));
    final iv        = enc.IV.fromLength(16);
    final encrypter = enc.Encrypter(enc.AES(key));
    return encrypter.decrypt64(text, iv: iv);
  } catch (_) {
    return text;
  }
}

Color _avatarColor(String name) =>
    Colors.primaries[name.hashCode.abs() % Colors.primaries.length];

// ─── ТОЧКА ВХОДА ─────────────────────────────────────────────────────────────
void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await AppSettings().load();
  SystemChrome.setSystemUIOverlayStyle(const SystemUiOverlayStyle(
    statusBarColor:          Colors.transparent,
    statusBarIconBrightness: Brightness.light,
  ));
  await Supabase.initialize(url: supabaseUrl, anonKey: supabaseKey);
  runApp(const MeowApp());
}

// ─── APP ─────────────────────────────────────────────────────────────────────
class MeowApp extends StatefulWidget {
  const MeowApp({super.key});

  @override
  State<MeowApp> createState() => _MeowAppState();
}

class _MeowAppState extends State<MeowApp> {
  final _s = AppSettings();

  @override
  void initState() {
    super.initState();
    _s.addListener(() => setState(() {}));
  }

  @override
  Widget build(BuildContext context) {
    final dark = _s.darkMode;
    final accent = _s.accent;

    final bgColor      = dark ? const Color(0xFF0A0A0F) : const Color(0xFFF5F5F5);
    final surfaceColor = dark ? const Color(0xFF141420) : const Color(0xFFFFFFFF);
    final cardColor    = dark ? const Color(0xFF1C1C2E) : const Color(0xFFEEEEEE);
    final textColor    = dark ? const Color(0xFFEEEEFF) : const Color(0xFF111111);
    final hintColor    = dark ? const Color(0xFF7070A0) : const Color(0xFF888888);

    return MaterialApp(
      title:                      'Meow',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness:              dark ? Brightness.dark : Brightness.light,
        scaffoldBackgroundColor: bgColor,
        primaryColor:            accent,
        colorScheme: ColorScheme(
          brightness: dark ? Brightness.dark : Brightness.light,
          primary:    accent,
          secondary:  accent,
          surface:    surfaceColor,
          error:      Colors.red,
          onPrimary:  Colors.white,
          onSecondary: Colors.white,
          onSurface:  textColor,
          onError:    Colors.white,
        ),
        appBarTheme: AppBarTheme(
          backgroundColor: dark ? const Color(0xFF0A0A0F) : accent,
          elevation:       0,
          titleTextStyle: TextStyle(
            color:         Colors.white,
            fontSize:      20,
            fontWeight:    FontWeight.w700,
            letterSpacing: -0.5,
          ),
          iconTheme: const IconThemeData(color: Colors.white),
        ),
        inputDecorationTheme: InputDecorationTheme(
          filled:    true,
          fillColor: cardColor,
          hintStyle: TextStyle(color: hintColor),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(24),
            borderSide:   BorderSide.none,
          ),
          contentPadding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
        ),
        floatingActionButtonTheme: FloatingActionButtonThemeData(
          backgroundColor: accent,
          foregroundColor: Colors.white,
        ),
        drawerTheme: DrawerThemeData(backgroundColor: surfaceColor),
      ),
      home: const MainScreen(),
    );
  }
}

// ─── ГЛАВНЫЙ ЭКРАН ───────────────────────────────────────────────────────────
class MainScreen extends StatefulWidget {
  const MainScreen({super.key});

  @override
  State<MainScreen> createState() => _MainScreenState();
}

class _MainScreenState extends State<MainScreen> {
  final _s     = AppSettings();
  String       _nick  = 'User';
  List<String> _chats = [];

  @override
  void initState() {
    super.initState();
    _load();
    _s.addListener(() => setState(() {}));
  }

  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    setState(() {
      _nick  = prefs.getString('nickname') ?? 'User';
      _chats = prefs.getStringList('chats') ?? [];
    });
  }

  Future<void> _saveChats() async =>
      (await SharedPreferences.getInstance()).setStringList('chats', _chats);

  Future<void> _saveNick(String nick) async =>
      (await SharedPreferences.getInstance()).setString('nickname', nick);

  // ── Диалог добавления чата ───────────────────────────────────────────────
  void _showAddChat() {
    final idCtrl  = TextEditingController();
    final keyCtrl = TextEditingController();
    showModalBottomSheet(
      context:            context,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(24, 24, 24, MediaQuery.of(ctx).viewInsets.bottom + 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Новый чат', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            Text('ID и ключ должны совпадать у обоих',
                style: TextStyle(fontSize: 13, color: Theme.of(ctx).hintColor)),
            const SizedBox(height: 16),
            TextField(controller: idCtrl, autofocus: true,
                decoration: const InputDecoration(hintText: 'ID чата (chat_key)')),
            const SizedBox(height: 10),
            TextField(controller: keyCtrl,
                decoration: const InputDecoration(hintText: 'Ключ шифрования (необязательно)')),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity, height: 50,
              child: ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: _s.accent,
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
                ),
                onPressed: () async {
                  final id = idCtrl.text.trim();
                  if (id.isEmpty) return;
                  final entry = '$id:${keyCtrl.text.trim()}';
                  if (!_chats.contains(entry)) {
                    _chats.add(entry);
                    await _saveChats();
                    setState(() {});
                  }
                  Navigator.pop(ctx);
                },
                child: const Text('Добавить',
                    style: TextStyle(fontWeight: FontWeight.w700, color: Colors.white)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  // ── Диалог настроек ──────────────────────────────────────────────────────
  void _showSettings() {
    Navigator.push(context, MaterialPageRoute(builder: (_) => SettingsScreen(nick: _nick, onNickChanged: (n) async {
      await _saveNick(n);
      setState(() => _nick = n);
    })));
  }

  void _deleteChat(int i) async {
    _chats.removeAt(i);
    await _saveChats();
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Meow'),
        actions: [
          GestureDetector(
            onTap: _showSettings,
            child: Padding(
              padding: const EdgeInsets.only(right: 12),
              child: CircleAvatar(
                radius: 18,
                backgroundColor: Colors.white24,
                child: Text(_nick[0].toUpperCase(),
                    style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700)),
              ),
            ),
          ),
        ],
      ),
      drawer: _buildDrawer(),
      body: _chats.isEmpty
          ? Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.lock_outline, size: 64,
                      color: Theme.of(context).hintColor.withOpacity(0.3)),
                  const SizedBox(height: 16),
                  Text('Нет чатов',
                      style: TextStyle(fontSize: 18, color: Theme.of(context).hintColor)),
                  const SizedBox(height: 8),
                  Text('Нажми + чтобы добавить',
                      style: TextStyle(fontSize: 13, color: Theme.of(context).hintColor)),
                ],
              ),
            )
          : ListView.separated(
              padding:     const EdgeInsets.symmetric(vertical: 8),
              itemCount:   _chats.length,
              separatorBuilder: (_, __) => Divider(height: 1,
                  indent: 72, color: Theme.of(context).dividerColor),
              itemBuilder: (_, i) {
                final parts  = _chats[i].split(':');
                final chatId = parts[0];
                final encKey = parts.length > 1 ? parts[1] : '';
                return Dismissible(
                  key:       Key(_chats[i]),
                  direction: DismissDirection.endToStart,
                  onDismissed: (_) => _deleteChat(i),
                  background: Container(
                    alignment: Alignment.centerRight,
                    padding:   const EdgeInsets.only(right: 20),
                    color:     Colors.red.withOpacity(0.8),
                    child:     const Icon(Icons.delete_outline, color: Colors.white),
                  ),
                  child: ListTile(
                    onTap: () => Navigator.push(context, MaterialPageRoute(
                      builder: (_) => ChatScreen(
                          roomName: chatId, encryptionKey: encKey, myNick: _nick),
                    )),
                    contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
                    leading: CircleAvatar(
                      radius: 26,
                      backgroundColor: _avatarColor(chatId).withOpacity(0.2),
                      child: Text(chatId[0].toUpperCase(),
                          style: TextStyle(color: _avatarColor(chatId),
                              fontWeight: FontWeight.w700, fontSize: 18)),
                    ),
                    title: Text(chatId,
                        style: const TextStyle(fontWeight: FontWeight.w600)),
                    subtitle: Text(
                      encKey.isNotEmpty ? '🔒 Зашифрован' : '🔓 Без шифрования',
                      style: TextStyle(
                          color: encKey.isNotEmpty ? _s.accent : Theme.of(context).hintColor,
                          fontSize: 12),
                    ),
                    trailing: const Icon(Icons.chevron_right),
                  ),
                );
              },
            ),
      floatingActionButton: FloatingActionButton(
        onPressed: _showAddChat,
        child: const Icon(Icons.add),
      ),
    );
  }

  Widget _buildDrawer() {
    return Drawer(
      child: Column(
        children: [
          UserAccountsDrawerHeader(
            decoration: BoxDecoration(color: _s.accent),
            currentAccountPicture: CircleAvatar(
              backgroundColor: Colors.white24,
              child: Text(_nick[0].toUpperCase(),
                  style: const TextStyle(fontSize: 24, color: Colors.white, fontWeight: FontWeight.w700)),
            ),
            accountName:  Text(_nick, style: const TextStyle(fontWeight: FontWeight.w700, color: Colors.white)),
            accountEmail: const Text('🔒 E2EE активно', style: TextStyle(color: Colors.white70, fontSize: 12)),
          ),
          ListTile(
            leading: const Icon(Icons.settings_outlined),
            title:   const Text('Настройки'),
            onTap: () { Navigator.pop(context); _showSettings(); },
          ),
          const Divider(),
          ListTile(
            leading: const Icon(Icons.delete_outline, color: Colors.redAccent),
            title:   const Text('Очистить все чаты', style: TextStyle(color: Colors.redAccent)),
            onTap: () async {
              (await SharedPreferences.getInstance()).remove('chats');
              setState(() => _chats = []);
              Navigator.pop(context);
            },
          ),
        ],
      ),
    );
  }
}

// ─── ЭКРАН НАСТРОЕК ──────────────────────────────────────────────────────────
class SettingsScreen extends StatefulWidget {
  final String nick;
  final void Function(String) onNickChanged;
  const SettingsScreen({super.key, required this.nick, required this.onNickChanged});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  final _s    = AppSettings();
  late final _nickCtrl = TextEditingController(text: widget.nick);

  @override
  void dispose() { _nickCtrl.dispose(); super.dispose(); }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Настройки')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [

          // ── Профиль ────────────────────────────────────────────────────
          _Section('Профиль', [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 4),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _nickCtrl,
                      decoration: const InputDecoration(
                        hintText: 'Твоё имя',
                        labelText: 'Имя пользователя',
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  ElevatedButton(
                    style: ElevatedButton.styleFrom(
                      backgroundColor: _s.accent,
                      foregroundColor: Colors.white,
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
                    ),
                    onPressed: () {
                      final n = _nickCtrl.text.trim();
                      if (n.isNotEmpty) widget.onNickChanged(n);
                    },
                    child: const Text('Сохранить'),
                  ),
                ],
              ),
            ),
          ]),

          const SizedBox(height: 16),

          // ── Тема ───────────────────────────────────────────────────────
          _Section('Тема', [
            SwitchListTile(
              value:    _s.darkMode,
              onChanged: (v) => setState(() => _s.setDarkMode(v)),
              title:    const Text('Тёмная тема'),
              secondary: Icon(_s.darkMode ? Icons.dark_mode : Icons.light_mode),
              activeColor: _s.accent,
            ),
          ]),

          const SizedBox(height: 16),

          // ── Акцентный цвет ─────────────────────────────────────────────
          _Section('Цвет акцента', [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: Wrap(
                spacing: 12,
                runSpacing: 12,
                children: List.generate(AppSettings.accentColors.length, (i) {
                  final selected = _s.accentIndex == i;
                  return GestureDetector(
                    onTap: () => setState(() => _s.setAccent(i)),
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 200),
                      width:  44,
                      height: 44,
                      decoration: BoxDecoration(
                        color:  AppSettings.accentColors[i],
                        shape:  BoxShape.circle,
                        border: selected ? Border.all(color: Colors.white, width: 3) : null,
                        boxShadow: selected
                            ? [BoxShadow(color: AppSettings.accentColors[i].withOpacity(0.6), blurRadius: 8)]
                            : null,
                      ),
                      child: selected
                          ? const Icon(Icons.check, color: Colors.white, size: 20)
                          : null,
                    ),
                  );
                }),
              ),
            ),
          ]),

          const SizedBox(height: 16),

          // ── Шрифт ──────────────────────────────────────────────────────
          _Section('Размер шрифта сообщений', [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
              child: Column(
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      const Text('A', style: TextStyle(fontSize: 12)),
                      Text('${_s.fontSize.round()} px',
                          style: TextStyle(color: _s.accent, fontWeight: FontWeight.w700)),
                      const Text('A', style: TextStyle(fontSize: 22)),
                    ],
                  ),
                  Slider(
                    value:    _s.fontSize,
                    min:      11,
                    max:      22,
                    divisions: 11,
                    activeColor: _s.accent,
                    onChanged: (v) => setState(() => _s.setFontSize(v)),
                  ),
                  // Превью
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color:        _s.accent.withOpacity(0.15),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text(
                      'Привет! Это пример текста сообщения.',
                      style: TextStyle(fontSize: _s.fontSize),
                    ),
                  ),
                ],
              ),
            ),
          ]),

          const SizedBox(height: 16),

          // ── Форма пузырей ──────────────────────────────────────────────
          _Section('Форма пузырей', [
            ...['Скруглённые', 'Острые', 'Telegram стиль'].asMap().entries.map((e) {
              return RadioListTile<int>(
                value:    e.key,
                groupValue: _s.bubbleStyle,
                onChanged: (v) => setState(() => _s.setBubbleStyle(v!)),
                title:    Text(e.value),
                activeColor: _s.accent,
              );
            }),
          ]),

          const SizedBox(height: 16),

          // ── Фон чата ───────────────────────────────────────────────────
          _Section('Фон чата', [
            ...['Без фона', 'Точки', 'Линии', 'Сетка'].asMap().entries.map((e) {
              return RadioListTile<int>(
                value:    e.key,
                groupValue: _s.chatBg,
                onChanged: (v) => setState(() => _s.setChatBg(v!)),
                title:    Text(e.value),
                activeColor: _s.accent,
              );
            }),
          ]),

          const SizedBox(height: 32),
        ],
      ),
    );
  }
}

// ─── ЭКРАН ЧАТА ──────────────────────────────────────────────────────────────
class ChatScreen extends StatefulWidget {
  final String roomName;
  final String encryptionKey;
  final String myNick;

  const ChatScreen({
    super.key,
    required this.roomName,
    required this.encryptionKey,
    required this.myNick,
  });

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final _s           = AppSettings();
  final _ctrl        = TextEditingController();
  final _supabase    = Supabase.instance.client;
  final _scroll      = ScrollController();
  bool  _showSend    = false;

  @override
  void initState() {
    super.initState();
    _ctrl.addListener(() {
      final has = _ctrl.text.isNotEmpty;
      if (has != _showSend) setState(() => _showSend = has);
    });
    _s.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _ctrl.dispose();
    _scroll.dispose();
    super.dispose();
  }

  void _send() async {
    final text = _ctrl.text.trim();
    if (text.isEmpty) return;
    _ctrl.clear();
    // sender_ — совместимо с itoryon/meow
    await _supabase.from('messages').insert({
      'sender_':  widget.myNick,
      'payload':  _encryptMsg(text, widget.encryptionKey),
      'chat_key': widget.roomName,
    });
  }

  // Рисуем фон чата
  Widget _buildChatBg(Widget child) {
    if (_s.chatBg == 0) return child;
    return CustomPaint(
      painter: _BgPainter(_s.chatBg, _s.accent.withOpacity(0.06)),
      child: child,
    );
  }

  @override
  Widget build(BuildContext context) {
    // stream() — совместимо с itoryon/meow
    final stream = _supabase
        .from('messages')
        .stream(primaryKey: ['id'])
        .eq('chat_key', widget.roomName)
        .order('id', ascending: false);

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(widget.roomName),
            Text(
              widget.encryptionKey.isNotEmpty ? '🔒 E2EE шифрование' : '🔓 Без шифрования',
              style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w400, color: Colors.white70),
            ),
          ],
        ),
      ),
      body: Column(
        children: [
          Expanded(
            child: _buildChatBg(
              StreamBuilder<List<Map<String, dynamic>>>(
                stream: stream,
                builder: (context, snap) {
                  if (snap.hasError) return Center(
                    child: Padding(
                      padding: const EdgeInsets.all(24),
                      child: Text(
                        'Ошибка: ${snap.error}\n\nПроверь RLS в Supabase.',
                        textAlign: TextAlign.center,
                        style: TextStyle(color: Theme.of(context).hintColor),
                      ),
                    ),
                  );
                  if (!snap.hasData) return Center(
                    child: CircularProgressIndicator(color: _s.accent),
                  );

                  final msgs = snap.data!;
                  if (msgs.isEmpty) return Center(
                    child: Text('Напиши первое сообщение',
                        style: TextStyle(color: Theme.of(context).hintColor)),
                  );

                  return ListView.builder(
                    controller: _scroll,
                    reverse:    true,
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 16),
                    itemCount:   msgs.length,
                    itemBuilder: (_, i) {
                      final m      = msgs[i];
                      // sender_ — совместимо с оригиналом
                      final sender = (m['sender_'] as String?) ?? '?';
                      final isMe   = sender == widget.myNick;
                      final text   = _decryptMsg((m['payload'] as String?) ?? '', widget.encryptionKey);
                      final showNick = i == msgs.length - 1 ||
                          msgs[i + 1]['sender_'] != sender;
                      String time = '';
                      if (m['created_at'] != null) {
                        time = DateTime.parse(m['created_at'])
                            .toLocal().toString().substring(11, 16);
                      }
                      return _Bubble(
                        text:      text,
                        sender:    sender,
                        time:      time,
                        isMe:      isMe,
                        showNick:  showNick && !isMe,
                        style:     _s.bubbleStyle,
                        fontSize:  _s.fontSize,
                        accent:    _s.accent,
                      );
                    },
                  );
                },
              ),
            ),
          ),
          // ── Поле ввода ────────────────────────────────────────────────
          Container(
            padding: EdgeInsets.fromLTRB(
              12, 8, 12, MediaQuery.of(context).padding.bottom + 8,
            ),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surface,
              border: Border(top: BorderSide(
                color: Theme.of(context).dividerColor, width: 0.5,
              )),
            ),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller:      _ctrl,
                    maxLines:        5,
                    minLines:        1,
                    textInputAction: TextInputAction.newline,
                    decoration: const InputDecoration(
                      hintText:       'Сообщение...',
                      contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                AnimatedSwitcher(
                  duration: const Duration(milliseconds: 200),
                  child: _showSend
                      ? GestureDetector(
                          key:   const ValueKey('send'),
                          onTap: _send,
                          child: Container(
                            width: 44, height: 44,
                            decoration: BoxDecoration(
                              color: _s.accent,
                              shape: BoxShape.circle,
                            ),
                            child: const Icon(Icons.send_rounded, color: Colors.white, size: 20),
                          ),
                        )
                      : const SizedBox(key: ValueKey('empty'), width: 44),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── ПУЗЫРЬ СООБЩЕНИЯ ────────────────────────────────────────────────────────
class _Bubble extends StatelessWidget {
  final String text;
  final String sender;
  final String time;
  final bool   isMe;
  final bool   showNick;
  final int    style;
  final double fontSize;
  final Color  accent;

  const _Bubble({
    required this.text,
    required this.sender,
    required this.time,
    required this.isMe,
    required this.showNick,
    required this.style,
    required this.fontSize,
    required this.accent,
  });

  BorderRadius _radius() {
    switch (style) {
      case 1: // острые
        return BorderRadius.only(
          topLeft:     Radius.circular(isMe ? 16 : 2),
          topRight:    Radius.circular(isMe ? 2  : 16),
          bottomLeft:  const Radius.circular(16),
          bottomRight: const Radius.circular(16),
        );
      case 2: // telegram
        return BorderRadius.only(
          topLeft:     const Radius.circular(18),
          topRight:    const Radius.circular(18),
          bottomLeft:  Radius.circular(isMe ? 18 : 4),
          bottomRight: Radius.circular(isMe ? 4  : 18),
        );
      default: // скруглённые
        return BorderRadius.only(
          topLeft:     Radius.circular(isMe ? 18 : (showNick ? 4 : 18)),
          topRight:    Radius.circular(isMe ? (showNick ? 4 : 18) : 18),
          bottomLeft:  const Radius.circular(18),
          bottomRight: const Radius.circular(18),
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final myColor    = accent.withOpacity(0.85);
    final otherColor = isDark ? const Color(0xFF252540) : const Color(0xFFEEEEEE);

    return Padding(
      padding: EdgeInsets.only(top: showNick ? 10 : 2, bottom: 2),
      child: Row(
        mainAxisAlignment: isMe ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (!isMe)
            Padding(
              padding: const EdgeInsets.only(right: 6, bottom: 2),
              child: showNick
                  ? CircleAvatar(
                      radius:          14,
                      backgroundColor: _avatarColor(sender).withOpacity(0.25),
                      child: Text(sender[0].toUpperCase(),
                          style: TextStyle(fontSize: 11, color: _avatarColor(sender),
                              fontWeight: FontWeight.w700)),
                    )
                  : const SizedBox(width: 28),
            ),
          Flexible(
            child: Container(
              constraints: BoxConstraints(
                  maxWidth: MediaQuery.of(context).size.width * 0.72),
              margin: EdgeInsets.only(left: isMe ? 60 : 0, right: isMe ? 0 : 60),
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              decoration: BoxDecoration(
                color:        isMe ? myColor : otherColor,
                borderRadius: _radius(),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize:       MainAxisSize.min,
                children: [
                  if (showNick)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 4),
                      child: Text(sender,
                          style: TextStyle(color: _avatarColor(sender),
                              fontSize: 12, fontWeight: FontWeight.w700)),
                    ),
                  Text(text,
                      style: TextStyle(
                        color:    isMe ? Colors.white : (isDark ? const Color(0xFFEEEEFF) : const Color(0xFF111111)),
                        fontSize: fontSize,
                      )),
                  const SizedBox(height: 4),
                  Align(
                    alignment: Alignment.bottomRight,
                    child: Text(time,
                        style: TextStyle(
                          color:    (isMe ? Colors.white : Colors.grey).withOpacity(0.7),
                          fontSize: 11,
                        )),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

// ─── ФОНОВЫЙ РИСУНОК ЧАТА ────────────────────────────────────────────────────
class _BgPainter extends CustomPainter {
  final int   type;
  final Color color;
  _BgPainter(this.type, this.color);

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()..color = color..strokeWidth = 1;
    switch (type) {
      case 1: // точки
        for (double x = 0; x < size.width; x += 24) {
          for (double y = 0; y < size.height; y += 24) {
            canvas.drawCircle(Offset(x, y), 1.5, paint);
          }
        }
        break;
      case 2: // линии
        for (double y = 0; y < size.height; y += 28) {
          canvas.drawLine(Offset(0, y), Offset(size.width, y), paint);
        }
        break;
      case 3: // сетка
        for (double x = 0; x < size.width; x += 28) {
          canvas.drawLine(Offset(x, 0), Offset(x, size.height), paint);
        }
        for (double y = 0; y < size.height; y += 28) {
          canvas.drawLine(Offset(0, y), Offset(size.width, y), paint);
        }
        break;
    }
  }

  @override
  bool shouldRepaint(_BgPainter old) => old.type != type || old.color != color;
}

// ─── СЕКЦИЯ В НАСТРОЙКАХ ─────────────────────────────────────────────────────
class _Section extends StatelessWidget {
  final String title;
  final List<Widget> children;
  const _Section(this.title, this.children);

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 4, bottom: 8),
          child: Text(title.toUpperCase(),
              style: TextStyle(
                fontSize:      11,
                fontWeight:    FontWeight.w700,
                letterSpacing: 1.2,
                color:         AppSettings().accent,
              )),
        ),
        Card(
          margin: EdgeInsets.zero,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
          child: Column(children: children),
        ),
      ],
    );
  }
}
