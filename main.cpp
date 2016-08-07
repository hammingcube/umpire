# include <iostream>
# include <chrono>
# include <thread>


using namespace std;
int main() {
  string s;
  cout << "Hello" << endl;
  while(cin >> s) {
  	std::this_thread::sleep_for(std::chrono::milliseconds(500));
    cout << s.size() << endl;
  }
}