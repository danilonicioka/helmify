package main
import (
	"html/template"
	"os"
	"fmt"
)
func main() {
	b, err := os.ReadFile("web/index.html")
	if err != nil { fmt.Println("read err:", err); return }
	_, err = template.New("html").Parse(string(b))
	if err != nil { fmt.Println("parse err:", err); return }
	fmt.Println("parse success")
}
