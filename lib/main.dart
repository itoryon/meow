import 'package:flutter/material.dart';
import 'package:supabase_flutter/supabase_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:encrypt/encrypt.dart' as enc;
import 'dart:async';

// ВСТАВЬ СВОИ ДАННЫЕ ИЗ SUPABASE
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
  Widget build(BuildContext context) => MaterialApp(
    debugShowCheckedModeBanner: false,
    theme: ThemeData.dark().copyWith(
      scaffoldBackgroundColor: const Color(0xFF121212),
    ),
    home: const MainScreen(),
  );
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
    _load();
  }

  _load() async {
    final prefs = await SharedPreferences.getInstance();
    setState(() {
      myNick = prefs.getString('nickname') ?? "User";
      myChats = prefs.getStringList('chats') ?? ["main:123"];
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text("Signal Clone v67")),
      body: ListView.builder(
        itemCount: myChats.length,
        itemBuilder: (c, i) {
          final p = myChats[i].split(':');
          return ListTile(
            leading: const Icon(Icons.security, color: Colors.blue),
            title: Text(p[0]),
            subtitle: const Text("Нажмите для входа"),
            onTap: () => Navigator.push(context, MaterialPageRoute(builder: (c) => ChatScreen(
              room: p[0], 
              pass: p.length > 1 ? p[1] : "", 
              nick: myNick
            ))),
          );
        },
      ),
    );
  }
}

class ChatScreen extends StatefulWidget {
  final String room, pass, nick;
  const ChatScreen({super.key, required this.room, required this.pass, required this.nick});
  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final _controller = TextEditingController();
  final _sb = Supabase.instance.client;
  List<Map<String, dynamic>> _msgs = [];
  String _status = "Загрузка...";
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
      final res = await _sb.from('messages')
          .select()
          .eq('chat_key', widget.room)
          .order('id', ascending: false)
          .limit(30);
      
      if (mounted) {
        setState(() {
          _msgs = List<Map<String, dynamic>>.from(res);
          _loading = false;
          _status = "OK. Найдено сообщений: ${_msgs.length}";
        });
      }
    } catch (e) {
      if (mounted) setState(() => _status = "Ошибка БД: $e");
    }
  }

  void _send() async {
    if (_controller.text.isEmpty) return;
    final raw = _controller.text;
    _controller.clear();

    String toSend = raw;
    if (widget.pass.isNotEmpty) {
      try {
        final key = enc.Key.fromUtf8(widget.pass.padRight(32).substring(0, 32));
        final iv = enc.IV.fromLength(16);
        final encrypter = enc.Encrypter(enc.AES(key));
        toSend = encrypter.encrypt(raw, iv: iv).base64;
      } catch (e) { print(e); }
    }

    try {
      // ИСПОЛЬЗУЕМ ВАШИ ПОЛЯ: sender, chat_key, payload
      await _sb.from('messages').insert({
        'sender': widget.nick, 
        'payload': toSend,
        'chat_key': widget.room,
      });
      _fetch();
    } catch (e) {
      setState(() => _status = "Ошибка отправки: $e");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.room)),
      body: Column(children: [
        // Полоска статуса для отладки
        Container(
          width: double.infinity,
          color: Colors.blue.withOpacity(0.1),
          padding: const EdgeInsets.all(4),
          child: Text(_status, style: const TextStyle(fontSize: 10, color: Colors.blueAccent)),
        ),
        Expanded(
          child: _loading 
            ? const Center(child: CircularProgressIndicator())
            : _msgs.isEmpty 
              ? const Center(child: Text("Тут пока ничего нет..."))
              : ListView.builder(
                  reverse: true,
                  itemCount: _msgs.length,
                  itemBuilder: (c, i) {
                    final m = _msgs[i];
                    bool isMe = m['sender'] == widget.nick;
                    return Align(
                      alignment: isMe ? Alignment.centerRight : Alignment.centerLeft,
                      child: Container(
                        padding: const EdgeInsets.all(12),
                        margin: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                        decoration: BoxDecoration(
                          color: isMe ? Colors.blue[800] : const Color(0xFF2D2D2D),
                          borderRadius: BorderRadius.circular(15),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            if(!isMe) Text(m['sender'] ?? "Аноним", style: const TextStyle(fontSize: 10, color: Colors.blueGrey, fontWeight: FontWeight.bold)),
                            Text(m['payload'] ?? ""), 
                          ],
                        ),
                      ),
                    );
                  },
                ),
        ),
        Padding(
          padding: const EdgeInsets.all(8.0),
          child: Row(children: [
            Expanded(child: TextField(controller: _controller, decoration: const InputDecoration(hintText: "Написать...", border: InputBorder.none))),
            IconButton(icon: const Icon(Icons.send, color: Colors.blue), onPressed: _send),
          ]),
        )
      ]),
    );
  }
}
