import 'package:flutter/material.dart';
import 'package:supabase_flutter/supabase_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:encrypt/encrypt.dart' as enc;
import 'dart:async';

// ВСТАВЬ СВОИ ДАННЫЕ СЮДА
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

  void _showAddChat() {
    final idC = TextEditingController();
    final keyC = TextEditingController();
    showDialog(
      context: context,
      builder: (c) => AlertDialog(
        title: const Text("Новый чат"),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(controller: idC, decoration: const InputDecoration(hintText: "ID (напр. main)")),
          TextField(controller: keyC, decoration: const InputDecoration(hintText: "Ключ (любой)")),
        ]),
        actions: [
          ElevatedButton(onPressed: () async {
            if(idC.text.isEmpty) return;
            final prefs = await SharedPreferences.getInstance();
            myChats.add("${idC.text}:${keyC.text}");
            await prefs.setStringList('chats', myChats);
            setState((){});
            Navigator.pop(c);
          }, child: const Text("OK"))
        ],
      )
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text("Signal Clone")),
      body: ListView.builder(
        itemCount: myChats.length,
        itemBuilder: (c, i) {
          final parts = myChats[i].split(':');
          return ListTile(
            title: Text(parts[0]),
            onTap: () => Navigator.push(context, MaterialPageRoute(builder: (c) => ChatScreen(
              roomName: parts[0], 
              encKey: parts.length > 1 ? parts[1] : "", 
              myNick: myNick
            ))),
          );
        },
      ),
      floatingActionButton: FloatingActionButton(onPressed: _showAddChat, child: const Icon(Icons.add)),
    );
  }
}

class ChatScreen extends StatefulWidget {
  final String roomName, encKey, myNick;
  const ChatScreen({super.key, required this.roomName, required this.encKey, required this.myNick});
  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final _msgC = TextEditingController();
  final _sb = Supabase.instance.client;
  List<Map<String, dynamic>> _msgs = [];
  bool _loading = true;
  Timer? _t;

  @override
  void initState() {
    super.initState();
    _fetch();
    _t = Timer.periodic(const Duration(seconds: 2), (t) => _fetch());
  }

  @override
  void dispose() { _t?.cancel(); super.dispose(); }

  Future<void> _fetch() async {
    try {
      final res = await _sb.from('messages').select().eq('chat_key', widget.roomName).order('id', ascending: false).limit(30);
      if (mounted) setState(() { _msgs = List<Map<String, dynamic>>.from(res); _loading = false; });
    } catch (e) { print("Error: $e"); }
  }

  String _decrypt(dynamic payload) {
    final text = payload?.toString() ?? "";
    if (widget.encKey.isEmpty || text.isEmpty) return text;
    try {
      final key = enc.Key.fromUtf8(widget.encKey.padRight(32).substring(0, 32));
      final iv = enc.IV.fromLength(16);
      final encrypter = enc.Encrypter(enc.AES(key));
      return encrypter.decrypt64(text, iv: iv);
    } catch (e) {
      return "🔒 $text"; // Показываем зашифрованный текст, если не удалось расшифровать
    }
  }

  void _send() async {
    if (_msgC.text.isEmpty) return;
    final raw = _msgC.text;
    _msgC.clear();

    String toSend = raw;
    if (widget.encKey.isNotEmpty) {
      final key = enc.Key.fromUtf8(widget.encKey.padRight(32).substring(0, 32));
      final iv = enc.IV.fromLength(16);
      final encrypter = enc.Encrypter(enc.AES(key));
      toSend = encrypter.encrypt(raw, iv: iv).base64;
    }

    await _sb.from('messages').insert({
      'sender_': widget.myNick,
      'payload': toSend,
      'chat_key': widget.roomName,
    });
    _fetch();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.roomName)),
      body: Column(children: [
        Expanded(
          child: _loading 
            ? const Center(child: CircularProgressIndicator())
            : _msgs.isEmpty 
              ? const Center(child: Text("Пусто. Напишите что-нибудь!"))
              : ListView.builder(
                  reverse: true,
                  itemCount: _msgs.length,
                  itemBuilder: (c, i) {
                    final m = _msgs[i];
                    bool isMe = m['sender_'] == widget.myNick;
                    return Container(
                      padding: const EdgeInsets.all(10),
                      margin: const EdgeInsets.all(5),
                      alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
                      child: Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: isMe ? Colors.blue : Colors.grey[800],
                          borderRadius: BorderRadius.circular(15)
                        ),
                        child: Text(_decrypt(m['payload'])),
                      ),
                    );
                  },
                ),
        ),
        Padding(
          padding: const EdgeInsets.all(8.0),
          child: Row(children: [
            Expanded(child: TextField(controller: _msgC, decoration: const InputDecoration(hintText: "Текст..."))),
            IconButton(icon: const Icon(Icons.send), onPressed: _send),
          ]),
        )
      ]),
    );
  }
}
