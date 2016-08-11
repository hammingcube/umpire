INPUT_TXT = String.raw`abc
hello
`



/// GOLANG CODE

GO_CODE = String.raw`
package main

import (
    "bufio"
    "fmt"
    "os"
)

func main() {
  scanner := bufio.NewScanner(os.Stdin)
  for scanner.Scan() {
      fmt.Println(len(scanner.Text()))
  }
}`



/// C++ CODE

CPP_CODE = String.raw`
# include <iostream>
using namespace std;
int main() {
	string s;
	while(cin >> s) {
	    cout << s.size() << endl;
	}
	return 0;		
}`



/// JAVASCRIPT CODE 

JS_CODE = String.raw`
var readline = require('readline');
var rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

rl.on('line', function(line){
    console.log(line.length);
})`




/// PYTHON CODE

PY_CODE = String.raw`
import sys

for line in sys.stdin:
  line = line.strip().rstrip()
  print(len(line))
`