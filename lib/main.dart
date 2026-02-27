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

// Утилита для генерации цвета по нику
Color _getAvatarColor(String name) {
  final int hash = name.hashCode;
  return Colors.primaries[hash.abs() % Colors.primaries.length];
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
            TextField(controller: idController, decoration: const InputDecoration(labelText: "ID чата")),
            TextField(controller: keyController, decoration: const InputDecoration(labelText: "Ключ шифрования")),
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Signal'), backgroundColor: const Color(0xFF121212)),
      drawer: Drawer(
        child: Column(
          children: [
            UserAccountsDrawerHeader(
              currentAccountPicture: CircleAvatar(
                backgroundColor: _getAvatarColor(myNick),
                child: Text(myNick[0].toUpperCase(), style: const TextStyle(fontSize: 24, color: Colors.white)),
              ),
              accountName: Text(myNick),
              accountEmail: const Text("ID: Protected"),
              decoration: const BoxDecoration(color: Color(0xFF1C1C1C)),
            ),
            ListTile(
              leading: const Icon(Icons.person),
              title: const Text("Профиль"),
              onTap: () {
                // Здесь вызов диалога ника как в v54
              },
            ),
          ],
        ),
      ),
      body: ListView.builder(
        itemCount: myChats.length,
        itemBuilder: (context, index) {
          final parts = myChats[index].split(':');
          final name = parts[0];
          return ListTile(
            leading: CircleAvatar(backgroundColor: _getAvatarColor(name), child: Text(name[0].toUpperCase())),
            title: Text(name),
            subtitle: const Text("Нажмите для входа"),
            onTap: () => Navigator.push(context, MaterialPageRoute(builder: (context) => ChatScreen(roomName: name, encryptionKey: parts[1], myNick: myNick))),
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
  bool _isTyping = false;

  @override
  void initState() {
    super.initState();
    _msgController.addListener(() => setState(() => _isTyping = _msgController.text.isNotEmpty));
  }

  String _decrypt(String text) {
    if (widget.encryptionKey.isEmpty) return text;
    try {
      final key = enc.Key.fromUtf8(widget.encryptionKey.padRight(32).substring(0, 32));
      final iv = enc.IV.fromLength(16);
      final encrypter = enc.Encrypter(enc.AES(key));
      return encrypter.decrypt64(text, iv: iv);
    } catch (e) {
      return text; // Если это старое незашифрованное сообщение
    }
  }

  void _send() async {
    final text = _msgController.text;
    _msgController.clear();
    
    // Шифруем
    final key = enc.Key.fromUtf8(widget.encryptionKey.padRight(32).substring(0, 32));
    final iv = enc.IV.fromLength(16);
    final encrypter = enc.Encrypter(enc.AES(key));
    final encrypted = encrypter.encrypt(text, iv: iv).base64;

    await _supabase.from('messages').insert({
      'sender_': widget.myNick, // Поправил на sender_
      'payload': encrypted,
      'chat_key': widget.roomName,
    });
  }

  @override
  Widget build(BuildContext context) {
    // Стрим с фильтром по твоим столбцам
    final stream = _supabase
        .from('messages')
        .stream(primaryKey: ['id'])
        .eq('chat_key', widget.roomName)
        .order('id', ascending: false);

    return Scaffold(
      appBar: AppBar(title: Text(widget.roomName), backgroundColor: const Color(0xFF121212)),
      body: Column(
        children: [
          Expanded(
            child: StreamBuilder<List<Map<String, dynamic>>>(
              stream: stream,
              builder: (context, snap) {
                if (snap.hasError) return Center(child: Text("Ошибка: ${snap.error}"));
                if (!snap.hasData) return const Center(child: CircularProgressIndicator());
                
                final msgs = snap.data!;
                return ListView.builder(
                  reverse: true,
                  itemCount: msgs.length,
                  itemBuilder: (context, i) {
                    final m = msgs[i];
                    bool isMe = m['sender_'] == widget.myNick;
                    return Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                      child: Row(
                        mainAxisAlignment: isMe ? MainAxisAlignment.end : MainAxisAlignment.start,
                        crossAxisAlignment: CrossAxisAlignment.end,
                        children: [
                          if (!isMe) CircleAvatar(radius: 12, backgroundColor: _getAvatarColor(m['sender_'] ?? "?"), child: Text(m['sender_']?[0].toUpperCase() ?? "?", style: const TextStyle(fontSize: 10))),
                          const SizedBox(width: 8),
                          Flexible(
                            child: Container(
                              padding: const EdgeInsets.all(12),
                              decoration: BoxDecoration(
                                color: isMe ? const Color(0xFF2090FF) : const Color(0xFF2D2D2D),
                                borderRadius: BorderRadius.circular(16),
                              ),
                              child: Text(_decrypt(m['payload'] ?? '')),
                            ),
                          ),
                        ],
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
                if (_isTyping) IconButton(icon: const Icon(Icons.send, color: Color(0xFF2090FF)), onPressed: _send),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
