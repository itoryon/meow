import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:supabase_flutter/supabase_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:encrypt/encrypt.dart' as enc;

// ════════════════════════════════════════════════════════════════════════════
//  КОНФИГ
// ════════════════════════════════════════════════════════════════════════════
const _supabaseUrl = 'https://ilszhdmqxsoixcefeoqa.supabase.co';
const _supabaseKey =
    'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc';

// ════════════════════════════════════════════════════════════════════════════
//  НАСТРОЙКИ (Singleton + ChangeNotifier)
// ════════════════════════════════════════════════════════════════════════════
class AppSettings extends ChangeNotifier {
  AppSettings._();
  static final instance = AppSettings._();

  bool   _dark        = true;
  int    _accentIdx   = 0;
  double _fontSize    = 15;
  int    _bubbleStyle = 0; // 0=скруглённые 1=острые 2=telegram
  int    _chatBg      = 0; // 0=нет 1=точки 2=линии 3=сетка

  bool   get dark        => _dark;
  int    get accentIdx   => _accentIdx;
  double get fontSize    => _fontSize;
  int    get bubbleStyle => _bubbleStyle;
  int    get chatBg      => _chatBg;

  static const accents = [
    Color(0xFF6C63FF), // фиолетовый
    Color(0xFF2090FF), // синий (оригинал)
    Color(0xFF00C896), // мятный
    Color(0xFFFF5F7E), // розовый
    Color(0xFFFF9500), // оранжевый
    Color(0xFF34C759), // зелёный
  ];

  Color get accent => accents[_accentIdx];

  Future<void> init() async {
    final p    = await SharedPreferences.getInstance();
    _dark        = p.getBool('dark')        ?? true;
    _accentIdx   = p.getInt('accentIdx')    ?? 0;
    _fontSize    = p.getDouble('fontSize')  ?? 15;
    _bubbleStyle = p.getInt('bubbleStyle')  ?? 0;
    _chatBg      = p.getInt('chatBg')       ?? 0;
    notifyListeners();
  }

  Future<void> setDark(bool v) async {
    _dark = v;
    (await SharedPreferences.getInstance()).setBool('dark', v);
    notifyListeners();
  }

  Future<void> setAccent(int v) async {
    _accentIdx = v;
    (await SharedPreferences.getInstance()).setInt('accentIdx', v);
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

// ════════════════════════════════════════════════════════════════════════════
//  ШИФРОВАНИЕ — идентично itoryon/meow
// ════════════════════════════════════════════════════════════════════════════
String _encrypt(String text, String rawKey) {
  if (rawKey.isEmpty) return text;
  final key       = enc.Key.fromUtf8(rawKey.padRight(32).substring(0, 32));
  final iv        = enc.IV.fromLength(16);
  final encrypter = enc.Encrypter(enc.AES(key));
  return encrypter.encrypt(text, iv: iv).base64;
}

String _decrypt(String text, String rawKey) {
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

// ════════════════════════════════════════════════════════════════════════════
//  УТИЛИТЫ
// ════════════════════════════════════════════════════════════════════════════
Color _avatarColor(String name) =>
    Colors.primaries[name.hashCode.abs() % Colors.primaries.length];

// ════════════════════════════════════════════════════════════════════════════
//  ТОЧКА ВХОДА
// ════════════════════════════════════════════════════════════════════════════
void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await AppSettings.instance.init();
  SystemChrome.setSystemUIOverlayStyle(const SystemUiOverlayStyle(
    statusBarColor: Colors.transparent,
  ));
  await Supabase.initialize(url: _supabaseUrl, anonKey: _supabaseKey);
  runApp(const MeowApp());
}

// ════════════════════════════════════════════════════════════════════════════
//  APP — перестраивается при смене темы
// ════════════════════════════════════════════════════════════════════════════
class MeowApp extends StatefulWidget {
  const MeowApp({super.key});
  @override
  State<MeowApp> createState() => _MeowAppState();
}

class _MeowAppState extends State<MeowApp> {
  final _s = AppSettings.instance;

  @override
  void initState() {
    super.initState();
    _s.addListener(_rebuild);
  }

  @override
  void dispose() {
    _s.removeListener(_rebuild); // ← убираем listener, нет утечки
    super.dispose();
  }

  void _rebuild() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final dark   = _s.dark;
    final accent = _s.accent;

    return MaterialApp(
      title:                      'Meow',
      debugShowCheckedModeBanner: false,
      theme: _buildTheme(dark, accent),
      home: const MainScreen(),
    );
  }

  ThemeData _buildTheme(bool dark, Color accent) {
    final bg      = dark ? const Color(0xFF0A0A0F) : const Color(0xFFF2F2F7);
    final surface = dark ? const Color(0xFF1C1C2E) : Colors.white;
    final card    = dark ? const Color(0xFF252535) : const Color(0xFFEEEEF3);
    final txtMain = dark ? const Color(0xFFEEEEFF) : const Color(0xFF0D0D1A);
    final txtHint = dark ? const Color(0xFF7070A0) : const Color(0xFF999999);

    return ThemeData(
      brightness:              dark ? Brightness.dark : Brightness.light,
      scaffoldBackgroundColor: bg,
      primaryColor:            accent,
      colorScheme: ColorScheme(
        brightness:  dark ? Brightness.dark : Brightness.light,
        primary:     accent,
        secondary:   accent,
        surface:     surface,
        error:       const Color(0xFFFF3B30),
        onPrimary:   Colors.white,
        onSecondary: Colors.white,
        onSurface:   txtMain,
        onError:     Colors.white,
      ),
      cardColor:     card,
      hintColor:     txtHint,
      dividerColor:  dark ? const Color(0xFF252535) : const Color(0xFFDDDDE8),
      appBarTheme: AppBarTheme(
        backgroundColor:  dark ? const Color(0xFF0A0A0F) : accent,
        elevation:        0,
        centerTitle:      false,
        systemOverlayStyle: SystemUiOverlayStyle(
          statusBarColor:           Colors.transparent,
          statusBarIconBrightness:  dark ? Brightness.light : Brightness.light,
        ),
        titleTextStyle: const TextStyle(
          color: Colors.white, fontSize: 19,
          fontWeight: FontWeight.w700, letterSpacing: -0.3,
        ),
        iconTheme: const IconThemeData(color: Colors.white),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled:    true,
        fillColor: card,
        hintStyle: TextStyle(color: txtHint),
        labelStyle: TextStyle(color: txtHint),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide:   BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide:   BorderSide(color: accent, width: 1.5),
        ),
        contentPadding: const EdgeInsets.symmetric(horizontal: 18, vertical: 14),
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith(
            (s) => s.contains(WidgetState.selected) ? accent : null),
        trackColor: WidgetStateProperty.resolveWith(
            (s) => s.contains(WidgetState.selected) ? accent.withOpacity(0.4) : null),
      ),
      sliderTheme: SliderThemeData(activeTrackColor: accent, thumbColor: accent),
      floatingActionButtonTheme: FloatingActionButtonThemeData(
        backgroundColor: accent, foregroundColor: Colors.white,
      ),
      drawerTheme: DrawerThemeData(backgroundColor: surface),
      listTileTheme: ListTileThemeData(iconColor: txtHint),
    );
  }
}

// ════════════════════════════════════════════════════════════════════════════
//  ГЛАВНЫЙ ЭКРАН
// ════════════════════════════════════════════════════════════════════════════
class MainScreen extends StatefulWidget {
  const MainScreen({super.key});
  @override
  State<MainScreen> createState() => _MainScreenState();
}

class _MainScreenState extends State<MainScreen> {
  final _s     = AppSettings.instance;
  String       _nick  = 'User';
  List<String> _chats = [];

  @override
  void initState() {
    super.initState();
    _s.addListener(_rebuild);
    _load();
  }

  @override
  void dispose() {
    _s.removeListener(_rebuild); // ← нет утечки
    super.dispose();
  }

  void _rebuild() => setState(() {});

  Future<void> _load() async {
    final p = await SharedPreferences.getInstance();
    setState(() {
      _nick  = p.getString('nickname') ?? 'User';
      _chats = p.getStringList('chats') ?? [];
    });
  }

  Future<void> _saveChats() async =>
      (await SharedPreferences.getInstance()).setStringList('chats', _chats);

  // ── Добавить чат ─────────────────────────────────────────────────────────
  void _showAddChat() {
    final idCtrl  = TextEditingController();
    final keyCtrl = TextEditingController();
    showModalBottomSheet(
      context:            context,
      isScrollControlled: true,
      backgroundColor:    Theme.of(context).colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(24, 20, 24, MediaQuery.of(ctx).viewInsets.bottom + 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Ручка
            Center(
              child: Container(
                width: 36, height: 4,
                margin: const EdgeInsets.only(bottom: 20),
                decoration: BoxDecoration(
                  color: Theme.of(ctx).dividerColor,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const Text('Новый чат',
                style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700)),
            const SizedBox(height: 6),
            Text('ID и ключ должны совпадать у обоих',
                style: TextStyle(fontSize: 13, color: Theme.of(ctx).hintColor)),
            const SizedBox(height: 18),
            TextField(
              controller:  idCtrl,
              autofocus:   true,
              decoration:  const InputDecoration(
                hintText:    'ID чата (chat_key)',
                prefixIcon:  Icon(Icons.tag),
              ),
            ),
            const SizedBox(height: 10),
            TextField(
              controller: keyCtrl,
              decoration: const InputDecoration(
                hintText:   'Ключ шифрования (необязательно)',
                prefixIcon: Icon(Icons.key_outlined),
              ),
            ),
            const SizedBox(height: 18),
            SizedBox(
              width: double.infinity, height: 50,
              child: ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: _s.accent,
                  foregroundColor: Colors.white,
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
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
                  if (ctx.mounted) Navigator.pop(ctx);
                },
                child: const Text('Добавить',
                    style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _deleteChat(int i) async {
    _chats.removeAt(i);
    await _saveChats();
    setState(() {});
  }

  void _openSettings() => Navigator.push(
    context,
    MaterialPageRoute(builder: (_) => SettingsScreen(
      nick: _nick,
      onNickChanged: (n) async {
        (await SharedPreferences.getInstance()).setString('nickname', n);
        setState(() => _nick = n);
      },
    )),
  );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Meow'),
        actions: [
          GestureDetector(
            onTap: _openSettings,
            child: Padding(
              padding: const EdgeInsets.only(right: 14),
              child: CircleAvatar(
                radius:          19,
                backgroundColor: Colors.white.withOpacity(0.2),
                child: Text(
                  _nick.isNotEmpty ? _nick[0].toUpperCase() : 'U',
                  style: const TextStyle(
                      color: Colors.white, fontWeight: FontWeight.w800, fontSize: 16),
                ),
              ),
            ),
          ),
        ],
      ),

      // ── Боковое меню ───────────────────────────────────────────────────
      drawer: Drawer(
        child: Column(
          children: [
            UserAccountsDrawerHeader(
              decoration:     BoxDecoration(color: _s.accent),
              currentAccountPicture: CircleAvatar(
                backgroundColor: Colors.white24,
                child: Text(_nick.isNotEmpty ? _nick[0].toUpperCase() : 'U',
                    style: const TextStyle(
                        fontSize: 26, color: Colors.white, fontWeight: FontWeight.w800)),
              ),
              accountName:  Text(_nick,
                  style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 16)),
              accountEmail: const Text('🔒 E2EE активно',
                  style: TextStyle(color: Colors.white70, fontSize: 12)),
            ),
            ListTile(
              leading: const Icon(Icons.settings_outlined),
              title:   const Text('Настройки'),
              onTap: () { Navigator.pop(context); _openSettings(); },
            ),
            const Divider(),
            ListTile(
              leading: const Icon(Icons.delete_sweep_outlined, color: Colors.redAccent),
              title:   const Text('Очистить все чаты',
                  style: TextStyle(color: Colors.redAccent)),
              onTap: () async {
                (await SharedPreferences.getInstance()).remove('chats');
                setState(() => _chats = []);
                if (mounted) Navigator.pop(context);
              },
            ),
          ],
        ),
      ),

      // ── Тело ───────────────────────────────────────────────────────────
      body: _chats.isEmpty
          ? Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.forum_outlined, size: 72,
                      color: theme.hintColor.withOpacity(0.25)),
                  const SizedBox(height: 16),
                  Text('Нет чатов',
                      style: TextStyle(fontSize: 20, color: theme.hintColor,
                          fontWeight: FontWeight.w600)),
                  const SizedBox(height: 8),
                  Text('Нажми + чтобы добавить',
                      style: TextStyle(fontSize: 14, color: theme.hintColor)),
                ],
              ),
            )
          : ListView.separated(
              padding:     const EdgeInsets.symmetric(vertical: 8),
              itemCount:   _chats.length,
              separatorBuilder: (_, __) => Divider(
                  height: 1, indent: 76, color: theme.dividerColor),
              itemBuilder: (_, i) {
                final parts  = _chats[i].split(':');
                final chatId = parts[0];
                final encKey = parts.length > 1 ? parts[1] : '';
                return Dismissible(
                  key:       Key(_chats[i]),
                  direction: DismissDirection.endToStart,
                  background: Container(
                    alignment: Alignment.centerRight,
                    padding:   const EdgeInsets.only(right: 24),
                    decoration: BoxDecoration(
                      color: Colors.red.withOpacity(0.85),
                    ),
                    child: const Icon(Icons.delete_outline,
                        color: Colors.white, size: 26),
                  ),
                  onDismissed: (_) => _deleteChat(i),
                  child: ListTile(
                    onTap: () => Navigator.push(context, MaterialPageRoute(
                      builder: (_) => ChatScreen(
                          roomName: chatId, encryptionKey: encKey, myNick: _nick),
                    )),
                    contentPadding:
                        const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                    leading: CircleAvatar(
                      radius:          26,
                      backgroundColor: _avatarColor(chatId).withOpacity(0.18),
                      child: Text(chatId[0].toUpperCase(),
                          style: TextStyle(
                              color:      _avatarColor(chatId),
                              fontWeight: FontWeight.w800,
                              fontSize:   18)),
                    ),
                    title: Text(chatId,
                        style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 16)),
                    subtitle: Text(
                      encKey.isNotEmpty ? '🔒 Зашифрован' : '🔓 Без шифрования',
                      style: TextStyle(
                          fontSize: 12,
                          color: encKey.isNotEmpty ? _s.accent : theme.hintColor),
                    ),
                    trailing:
                        Icon(Icons.chevron_right, color: theme.hintColor),
                  ),
                );
              },
            ),

      floatingActionButton: FloatingActionButton(
        onPressed: _showAddChat,
        child:     const Icon(Icons.edit_outlined),
      ),
    );
  }
}

// ════════════════════════════════════════════════════════════════════════════
//  ЭКРАН НАСТРОЕК
// ════════════════════════════════════════════════════════════════════════════
class SettingsScreen extends StatefulWidget {
  final String nick;
  final void Function(String) onNickChanged;

  const SettingsScreen({
    super.key,
    required this.nick,
    required this.onNickChanged,
  });

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  final _s = AppSettings.instance;
  late final _nickCtrl = TextEditingController(text: widget.nick);

  @override
  void initState() {
    super.initState();
    _s.addListener(_rebuild);
  }

  @override
  void dispose() {
    _s.removeListener(_rebuild); // ← нет утечки
    _nickCtrl.dispose();
    super.dispose();
  }

  void _rebuild() => setState(() {});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Настройки')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [

          // ── Профиль ────────────────────────────────────────────────────
          _SettingSection(title: 'ПРОФИЛЬ', children: [
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _nickCtrl,
                      decoration: const InputDecoration(
                        hintText:   'Твоё имя',
                        labelText:  'Имя в чатах',
                        prefixIcon: Icon(Icons.person_outline),
                      ),
                    ),
                  ),
                  const SizedBox(width: 10),
                  ElevatedButton(
                    style: ElevatedButton.styleFrom(
                      backgroundColor: _s.accent,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(14)),
                    ),
                    onPressed: () {
                      final n = _nickCtrl.text.trim();
                      if (n.isNotEmpty) widget.onNickChanged(n);
                    },
                    child: const Text('OK', style: TextStyle(fontWeight: FontWeight.w700)),
                  ),
                ],
              ),
            ),
          ]),

          const SizedBox(height: 20),

          // ── Тема ───────────────────────────────────────────────────────
          _SettingSection(title: 'ТЕМА', children: [
            SwitchListTile(
              value:     _s.dark,
              onChanged: (v) => _s.setDark(v),
              secondary: Icon(_s.dark ? Icons.dark_mode_outlined : Icons.light_mode_outlined),
              title:     const Text('Тёмная тема'),
            ),
          ]),

          const SizedBox(height: 20),

          // ── Цвет акцента ───────────────────────────────────────────────
          _SettingSection(title: 'ЦВЕТ АКЦЕНТА', children: [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: List.generate(AppSettings.accents.length, (i) {
                  final selected = _s.accentIdx == i;
                  return GestureDetector(
                    onTap: () => _s.setAccent(i),
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 200),
                      width:  42, height: 42,
                      decoration: BoxDecoration(
                        color:  AppSettings.accents[i],
                        shape:  BoxShape.circle,
                        border: selected
                            ? Border.all(color: Colors.white, width: 3)
                            : null,
                        boxShadow: selected
                            ? [BoxShadow(
                                color:      AppSettings.accents[i].withOpacity(0.55),
                                blurRadius: 10, spreadRadius: 1)]
                            : null,
                      ),
                      child: selected
                          ? const Icon(Icons.check, color: Colors.white, size: 18)
                          : null,
                    ),
                  );
                }),
              ),
            ),
          ]),

          const SizedBox(height: 20),

          // ── Размер шрифта ──────────────────────────────────────────────
          _SettingSection(title: 'РАЗМЕР ТЕКСТА', children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
              child: Column(
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('Аа', style: TextStyle(fontSize: 12, color: Theme.of(context).hintColor)),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                        decoration: BoxDecoration(
                          color:        _s.accent.withOpacity(0.15),
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: Text('${_s.fontSize.round()} px',
                            style: TextStyle(color: _s.accent,
                                fontWeight: FontWeight.w700, fontSize: 13)),
                      ),
                      Text('Аа', style: TextStyle(fontSize: 20, color: Theme.of(context).hintColor)),
                    ],
                  ),
                  Slider(
                    value:     _s.fontSize,
                    min:       11, max: 22, divisions: 11,
                    onChanged: (v) => _s.setFontSize(v),
                  ),
                  // Превью
                  Container(
                    width:   double.infinity,
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color:        _s.accent.withOpacity(0.12),
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: Text(
                      'Привет! Это пример текста.',
                      style: TextStyle(fontSize: _s.fontSize),
                      textAlign: TextAlign.center,
                    ),
                  ),
                  const SizedBox(height: 8),
                ],
              ),
            ),
          ]),

          const SizedBox(height: 20),

          // ── Форма пузырей ──────────────────────────────────────────────
          _SettingSection(title: 'ФОРМА ПУЗЫРЕЙ', children: [
            _RadioTile('Скруглённые', 0, _s.bubbleStyle, _s.setBubbleStyle),
            _RadioTile('Острые',      1, _s.bubbleStyle, _s.setBubbleStyle),
            _RadioTile('Telegram',    2, _s.bubbleStyle, _s.setBubbleStyle),
          ]),

          const SizedBox(height: 20),

          // ── Фон чата ───────────────────────────────────────────────────
          _SettingSection(title: 'ФОН ЧАТА', children: [
            _RadioTile('Без фона', 0, _s.chatBg, _s.setChatBg),
            _RadioTile('Точки',    1, _s.chatBg, _s.setChatBg),
            _RadioTile('Линии',    2, _s.chatBg, _s.setChatBg),
            _RadioTile('Сетка',    3, _s.chatBg, _s.setChatBg),
          ]),

          const SizedBox(height: 32),
        ],
      ),
    );
  }
}

// ════════════════════════════════════════════════════════════════════════════
//  ЭКРАН ЧАТА
// ════════════════════════════════════════════════════════════════════════════
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
  final _s       = AppSettings.instance;
  final _ctrl    = TextEditingController();
  final _supabase = Supabase.instance.client;
  final _scroll  = ScrollController();
  bool  _hasTxt  = false;

  @override
  void initState() {
    super.initState();
    _s.addListener(_rebuild);
    _ctrl.addListener(() {
      final has = _ctrl.text.isNotEmpty;
      if (has != _hasTxt) setState(() => _hasTxt = has);
    });
  }

  @override
  void dispose() {
    _s.removeListener(_rebuild); // ← нет утечки
    _ctrl.dispose();
    _scroll.dispose();
    super.dispose();
  }

  void _rebuild() => setState(() {});

  void _send() async {
    final text = _ctrl.text.trim();
    if (text.isEmpty) return;
    _ctrl.clear();
    // sender_ — совместимо с itoryon/meow (оригинал использует именно sender_)
    await _supabase.from('messages').insert({
      'sender_':  widget.myNick,
      'payload':  _encrypt(text, widget.encryptionKey),
      'chat_key': widget.roomName,
    });
  }

  @override
  Widget build(BuildContext context) {
    // stream() — совместимо с itoryon/meow
    final stream = _supabase
        .from('messages')
        .stream(primaryKey: ['id'])
        .eq('chat_key', widget.roomName)
        .order('id', ascending: false);

    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(widget.roomName),
            Text(
              widget.encryptionKey.isNotEmpty
                  ? '🔒 E2EE шифрование'
                  : '🔓 Без шифрования',
              style: const TextStyle(
                  fontSize: 11, color: Colors.white70, fontWeight: FontWeight.w400),
            ),
          ],
        ),
      ),
      body: Column(
        children: [
          // ── Сообщения ────────────────────────────────────────────────
          Expanded(
            child: CustomPaint(
              painter: _BgPainter(_s.chatBg, _s.accent.withOpacity(0.05)),
              child: StreamBuilder<List<Map<String, dynamic>>>(
                stream: stream,
                builder: (ctx, snap) {
                  if (snap.hasError) {
                    return Center(
                      child: Padding(
                        padding: const EdgeInsets.all(24),
                        child: Text(
                          'Ошибка: ${snap.error}\n\nПроверь RLS в Supabase.',
                          textAlign: TextAlign.center,
                          style: TextStyle(color: theme.hintColor),
                        ),
                      ),
                    );
                  }
                  if (!snap.hasData) {
                    return Center(
                        child: CircularProgressIndicator(color: _s.accent));
                  }
                  final msgs = snap.data!;
                  if (msgs.isEmpty) {
                    return Center(
                      child: Text('Напиши первое сообщение 👋',
                          style: TextStyle(color: theme.hintColor, fontSize: 15)),
                    );
                  }
                  return ListView.builder(
                    controller: _scroll,
                    reverse:    true,
                    padding: const EdgeInsets.symmetric(
                        horizontal: 10, vertical: 14),
                    itemCount:   msgs.length,
                    itemBuilder: (_, i) {
                      final m      = msgs[i];
                      final sender = (m['sender_'] as String?) ?? '?';
                      final isMe   = sender == widget.myNick;
                      final text   = _decrypt(
                          (m['payload'] as String?) ?? '', widget.encryptionKey);
                      // показываем ник если отправитель сменился
                      final showNick = !isMe && (
                        i == msgs.length - 1 ||
                        msgs[i + 1]['sender_'] != sender
                      );
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
                        showNick:  showNick,
                        style:     _s.bubbleStyle,
                        fontSize:  _s.fontSize,
                        accent:    _s.accent,
                        dark:      _s.dark,
                      );
                    },
                  );
                },
              ),
            ),
          ),

          // ── Поле ввода ───────────────────────────────────────────────
          Container(
            padding: EdgeInsets.fromLTRB(
                10, 8, 10, MediaQuery.of(context).padding.bottom + 8),
            decoration: BoxDecoration(
              color:  theme.colorScheme.surface,
              border: Border(top: BorderSide(color: theme.dividerColor, width: 0.5)),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Expanded(
                  child: TextField(
                    controller:      _ctrl,
                    maxLines:        5,
                    minLines:        1,
                    textInputAction: TextInputAction.newline,
                    decoration: const InputDecoration(
                      hintText:       'Сообщение...',
                      contentPadding: EdgeInsets.symmetric(
                          horizontal: 16, vertical: 10),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                AnimatedSwitcher(
                  duration: const Duration(milliseconds: 180),
                  transitionBuilder: (child, anim) => ScaleTransition(
                      scale: anim, child: child),
                  child: _hasTxt
                      ? GestureDetector(
                          key:   const ValueKey('send'),
                          onTap: _send,
                          child: Container(
                            width: 44, height: 44,
                            margin: const EdgeInsets.only(bottom: 2),
                            decoration: BoxDecoration(
                              color: _s.accent,
                              shape: BoxShape.circle,
                            ),
                            child: const Icon(Icons.send_rounded,
                                color: Colors.white, size: 20),
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

// ════════════════════════════════════════════════════════════════════════════
//  ПУЗЫРЬ СООБЩЕНИЯ
// ════════════════════════════════════════════════════════════════════════════
class _Bubble extends StatelessWidget {
  final String text;
  final String sender;
  final String time;
  final bool   isMe;
  final bool   showNick;
  final int    style;
  final double fontSize;
  final Color  accent;
  final bool   dark;

  const _Bubble({
    required this.text,
    required this.sender,
    required this.time,
    required this.isMe,
    required this.showNick,
    required this.style,
    required this.fontSize,
    required this.accent,
    required this.dark,
  });

  BorderRadius _radius() {
    switch (style) {
      case 1: // острые
        return BorderRadius.only(
          topLeft:     Radius.circular(isMe ? 18 : 4),
          topRight:    Radius.circular(isMe ? 4  : 18),
          bottomLeft:  const Radius.circular(18),
          bottomRight: const Radius.circular(18),
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
    final myBg    = accent.withOpacity(0.9);
    final otherBg = dark ? const Color(0xFF252540) : const Color(0xFFEEEEF3);
    final txtClr  = isMe
        ? Colors.white
        : (dark ? const Color(0xFFEEEEFF) : const Color(0xFF111111));

    return Padding(
      padding: EdgeInsets.only(top: showNick ? 10 : 2, bottom: 2),
      child: Row(
        mainAxisAlignment:  isMe ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          // Аватар собеседника
          if (!isMe)
            Padding(
              padding: const EdgeInsets.only(right: 6, bottom: 2),
              child: showNick
                  ? CircleAvatar(
                      radius:          14,
                      backgroundColor: _avatarColor(sender).withOpacity(0.2),
                      child: Text(sender[0].toUpperCase(),
                          style: TextStyle(
                              fontSize:   11,
                              color:      _avatarColor(sender),
                              fontWeight: FontWeight.w800)),
                    )
                  : const SizedBox(width: 28),
            ),

          // Пузырь
          Flexible(
            child: Container(
              constraints: BoxConstraints(
                  maxWidth: MediaQuery.of(context).size.width * 0.73),
              margin: EdgeInsets.only(
                  left: isMe ? 56 : 0, right: isMe ? 0 : 56),
              padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 9),
              decoration: BoxDecoration(
                color:        isMe ? myBg : otherBg,
                borderRadius: _radius(),
                boxShadow: [
                  BoxShadow(
                    color:      Colors.black.withOpacity(0.08),
                    blurRadius: 4, offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize:       MainAxisSize.min,
                children: [
                  if (showNick)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 3),
                      child: Text(sender,
                          style: TextStyle(
                              color:      _avatarColor(sender),
                              fontSize:   12,
                              fontWeight: FontWeight.w700)),
                    ),
                  Text(text,
                      style: TextStyle(color: txtClr, fontSize: fontSize, height: 1.35)),
                  const SizedBox(height: 3),
                  Align(
                    alignment: Alignment.bottomRight,
                    child: Text(time,
                        style: TextStyle(
                            color:    (isMe ? Colors.white : Colors.grey).withOpacity(0.65),
                            fontSize: 10)),
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

// ════════════════════════════════════════════════════════════════════════════
//  ФОН ЧАТА
// ════════════════════════════════════════════════════════════════════════════
class _BgPainter extends CustomPainter {
  final int   type;
  final Color color;
  const _BgPainter(this.type, this.color);

  @override
  void paint(Canvas canvas, Size size) {
    if (type == 0) return;
    final p = Paint()..color = color..strokeWidth = 1;
    switch (type) {
      case 1: // точки
        for (double x = 16; x < size.width; x += 24) {
          for (double y = 16; y < size.height; y += 24) {
            canvas.drawCircle(Offset(x, y), 1.5, p);
          }
        }
        break;
      case 2: // линии
        for (double y = 0; y < size.height; y += 28) {
          canvas.drawLine(Offset(0, y), Offset(size.width, y), p);
        }
        break;
      case 3: // сетка
        for (double x = 0; x < size.width; x += 28) {
          canvas.drawLine(Offset(x, 0), Offset(x, size.height), p);
        }
        for (double y = 0; y < size.height; y += 28) {
          canvas.drawLine(Offset(0, y), Offset(size.width, y), p);
        }
        break;
    }
  }

  @override
  bool shouldRepaint(_BgPainter o) => o.type != type || o.color != color;
}

// ════════════════════════════════════════════════════════════════════════════
//  ВСПОМОГАТЕЛЬНЫЕ ВИДЖЕТЫ
// ════════════════════════════════════════════════════════════════════════════
class _SettingSection extends StatelessWidget {
  final String       title;
  final List<Widget> children;
  const _SettingSection({required this.title, required this.children});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 4, bottom: 8),
          child: Text(title,
              style: TextStyle(
                  fontSize:      11,
                  fontWeight:    FontWeight.w700,
                  letterSpacing: 1.1,
                  color:         AppSettings.instance.accent)),
        ),
        Card(
          margin:   EdgeInsets.zero,
          color:    Theme.of(context).colorScheme.surface,
          shape:    RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
          elevation: 0,
          child: Column(children: children),
        ),
      ],
    );
  }
}

class _RadioTile extends StatelessWidget {
  final String   label;
  final int      value;
  final int      groupValue;
  final Future<void> Function(int) onChanged;

  const _RadioTile(this.label, this.value, this.groupValue, this.onChanged);

  @override
  Widget build(BuildContext context) {
    return RadioListTile<int>(
      value:      value,
      groupValue: groupValue,
      onChanged:  (v) => onChanged(v!),
      title:      Text(label),
      activeColor: AppSettings.instance.accent,
      dense:      true,
    );
  }
}
