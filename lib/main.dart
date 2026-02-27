import 'package:flutter/material.dart';
import 'package:supabase_flutter/supabase_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';

const supabaseUrl = 'https://ilszhdmqxsoixcefeoqa.supabase.co';
const supabaseKey = 'YOUR_KEY_HERE';

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

  // Загружаем ник и чаты из памяти
  _loadData() async {
    final prefs = await SharedPreferences.getInstance();
    setState(() {
      myNick = prefs.getString('nickname') ?? "User";
      myChats = prefs.getStringList('chats') ?? [];
    });
  }

  // Показываем окно создания чата
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

  // Окно смены профиля
  void _showProfileDialog() {
    final nickController = TextEditingController(text: myNick);
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text("Мой профиль"),
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
      appBar: AppBar(title: const Text('Signal'), backgroundColor: const Color(0xFF121212)),
      drawer: Drawer(
        child: Column(
          children: [
            UserAccountsDrawerHeader(
              currentAccountPicture: const CircleAvatar(backgroundColor: Colors.blueAccent, child: Icon(Icons.person, color: Colors.white)),
              accountName: Text(myNick),
              accountEmail: const Text("Supabase Auth Active"),
              decoration: const BoxDecoration(color: Color(0xFF1C1C1C)),
            ),
            ListTile(leading: const Icon(Icons.edit), title: const Text("Сменить ник"), onTap: () { Navigator.pop(context); _showProfileDialog(); }),
            ListTile(leading: const Icon(Icons.settings), title: const Text("Настройки"), subtitle: const Text("В разработке"), onTap: () {}),
          ],
        ),
      ),
      body: myChats.isEmpty 
        ? const Center(child: Text("Нажмите +, чтобы добавить чат"))
        : ListView.builder(
            itemCount: myChats.length,
            itemBuilder: (context, index) {
              final parts = myChats[index].split(':');
              return ListTile(
                leading: const CircleAvatar(backgroundColor: Color(0xFF2090FF), child: Icon(Icons.chat_bubble_outline)),
                title: Text(parts[0]),
                subtitle: const Text("Нажмите, чтобы войти"),
                onTap: () {
                  // Здесь будет переход в ChatScreen из прошлого кода
                },
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
