import 'package:flutter/material.dart';
import 'package:supabase_flutter/supabase_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:encrypt/encrypt.dart' as enc;

const supabaseUrl = 'https://ilszhdmqxsoixcefeoqa.supabase.co';
const supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Imlsc3poZG1xeHNvaXhjZWZlb3FhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjA2NjA4NDMsImV4cCI6MjA3NjIzNjg0M30.aJF9c3RaNvAk4_9nLYhQABH3pmYUcZ0q2udf2LoA6Sc';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Supabase.initialize(url: supabaseUrl, anonKey: supabaseKey);
  runApp(const SignalApp());
}

class SignalApp extends StatelessWidget {
  const SignalApp({super.key});
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: ThemeData.dark().copyWith(
        scaffoldBackgroundColor: const Color(0xFF121212),
        primaryColor: const Color(0xFF2090FF),
      ),
      home: const MainScreen(),
    );
  }
}

class MainScreen extends StatefulWidget {
  const MainScreen({super.key});
  @override
  State<MainScreen> createState() => _MainScreenState();
}

class _MainScreenState extends State<MainScreen> {
  String myNick = "User";
  List<String> myChats = [];

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  _loadData() async {
    final prefs = await SharedPreferences.getInstance();
    setState(() {
      myNick = prefs.getString('nickname') ?? "User";
      myChats = prefs.getStringList('chats') ?? [];
    });
  }

  void _showAddChatDialog() {
    final idController = TextEditingController();
    final keyController = TextEditingController();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text("Новый чат"),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(controller: idController, decoration: const InputDecoration(labelText: "ID чата (латиница)")),
            TextField(controller: keyController, decoration: const InputDecoration(labelText: "Ключ (любой текст)")),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text("Отмена")),
          ElevatedButton(
            onPressed: () async {
              if (idController.text.isNotEmpty) {
                final prefs = await SharedPreferences.getInstance();
                myChats.add("${idController.text}:${keyController.text}");
                await prefs.setStringList('chats', myChats);
                setState(() {});
                Navigator.pop(context);
              }
            },
            child: const Text("Добавить"),
          ),
        ],
      ),
    );
  }

  void _showProfileDialog() {
    final nickController = TextEditingController(text: myNick);
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text("Профиль"),
        content: TextField(controller: nickController, decoration: const InputDecoration(labelText: "Ваш ник")),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text("Закрыть")),
          ElevatedButton(
            onPressed: () async {
              final prefs = await SharedPreferences.getInstance();
              await prefs.setString('nickname', nickController.text);
              setState(() { myNick = nickController.text; });
              Navigator.pop(context);
            },
            child: const Text("Сохранить"),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Signal'), backgroundColor: const Color(0xFF121212), elevation: 0),
      drawer: Drawer(
        child: Column(
          children: [
            UserAccountsDrawerHeader(
              currentAccountPicture: const CircleAvatar(backgroundColor: Color(0xFF2090FF), child: Icon(Icons.person, color: Colors.white, size: 40)),
              accountName: Text(myNick, style: const TextStyle(fontWeight: FontWeight.bold)),
              accountEmail: const Text("E2EE Защита включена"),
              decoration: const BoxDecoration(color: Color(0xFF1C1C1C)),
            ),
            ListTile(
              leading: const Icon(Icons.edit),
              title: const Text("Изменить ник"),
              onTap: () { Navigator.pop(context); _showProfileDialog(); },
            ),
            ListTile(
              leading: const Icon(Icons.settings),
              title: const Text("Настройки"),
              subtitle: const Text("В разработке", style: TextStyle(fontSize: 10)),
              onTap: () {},
            ),
          ],
        ),
      ),
      body: myChats.isEmpty 
        ? const Center(child: Text("Нет чатов. Нажмите +"))
        : ListView.builder(
            itemCount: myChats.length,
            itemBuilder: (context, index) {
              final parts = myChats[index].split(':');
              final name = parts[0];
              final key = parts.length > 1 ? parts[1] : "";
              return ListTile(
                leading: const CircleAvatar(backgroundColor: Color(0xFF2D2D2D), child: Icon(Icons.chat, color: Color(0xFF2090FF))),
                title: Text(name),
                subtitle: const Text("Нажмите, чтобы войти"),
                onTap: () => Navigator.push(context, MaterialPageRoute(builder: (context) => ChatScreen(roomName: name, encryptionKey: key, myNick: myNick))),
              );
            },
          ),
      floatingActionButton: FloatingActionButton(
        onPressed: _showAddChatDialog,
        backgroundColor: const Color(0xFF2090FF),
        child: const Icon(Icons.add, color: Colors.white),
      ),
    );
  }
}

class ChatScreen extends StatefulWidget {
  final String roomName;
  final String encryptionKey;
  final String myNick;
  const ChatScreen({super.key, required this.roomName, required this.encryptionKey, required this.myNick});

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final _msgController = TextEditingController();
  final _supabase = Supabase.instance.client;
  bool _showSend = false;

  @override
  void initState() {
    super.initState();
    _msgController.addListener(() => setState(() => _showSend = _msgController.text.isNotEmpty));
  }

  // --- ЛОГИКА ШИФРОВАНИЯ ---
  String _encrypt(String text) {
    if (widget.encryptionKey.isEmpty) return text;
    final key = enc.Key.fromUtf8(widget.encryptionKey.padRight(32).substring(0, 32));
    final iv = enc.IV.fromLength(16);
    final encrypter = enc.Encrypter(enc.AES(key));
    return encrypter.encrypt(text, iv: iv).base64;
  }

  String _decrypt(String base64Text) {
    if (widget.encryptionKey.isEmpty) return base64Text;
    try {
      final key = enc.Key.fromUtf8(widget.encryptionKey.padRight(32).substring(0, 32));
      final iv = enc.IV.fromLength(16);
      final encrypter = enc.Encrypter(enc.AES(key));
      return encrypter.decrypt64(base64Text, iv: iv);
    } catch (e) {
      return "[Ошибка: неверный ключ]";
    }
  }

  void _send() async {
    final text = _msgController.text;
    _msgController.clear();
    await _supabase.from('messages').insert({
      'sender': widget.myNick,
      'payload': _encrypt(text),
      'chat_key': widget.roomName
    });
  }

  @override
  Widget build(BuildContext context) {
    final stream = _supabase.from('messages').stream(primaryKey: ['id']).eq('chat_key', widget.roomName).order('id', ascending: false);

    return Scaffold(
      appBar: AppBar(title: Text(widget.roomName), backgroundColor: const Color(0xFF121212)),
      body: Column(
        children: [
          Expanded(
            child: StreamBuilder<List<Map<String, dynamic>>>(
              stream: stream,
              builder: (context, snap) {
                if (!snap.hasData) return const Center(child: CircularProgressIndicator());
                return ListView.builder(
                  reverse: true,
                  itemCount: snap.data!.length,
                  itemBuilder: (context, i) {
                    final m = snap.data![i];
                    bool isMe = m['sender'] == widget.myNick;
                    return Align(
                      alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
                      child: Container(
                        margin: const EdgeInsets.symmetric(vertical: 4, horizontal: 12),
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: isMe ? const Color(0xFF2090FF) : const Color(0xFF2D2D2D),
                          borderRadius: BorderRadius.circular(16),
                        ),
                        child: Text(_decrypt(m['payload'] ?? '')),
                      ),
                    );
                  },
                );
              },
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(8.0),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _msgController,
                    decoration: InputDecoration(
                      hintText: "Сообщение",
                      fillColor: const Color(0xFF2D2D2D),
                      filled: true,
                      border: OutlineInputBorder(borderRadius: BorderRadius.circular(25), borderSide: BorderSide.none),
                    ),
                  ),
                ),
                if (_showSend) IconButton(icon: const Icon(Icons.send, color: Color(0xFF2090FF)), onPressed: _send),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
