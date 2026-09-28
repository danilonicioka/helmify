package main
import (
	"html/template"
	"os"
	"fmt"
	"bytes"
)
func main() {
	b, err := os.ReadFile("web/index.html")
	if err != nil { fmt.Println("read err:", err); return }
	tmpl, err := template.New("html").Parse(string(b))
	if err != nil { fmt.Println("parse err:", err); return }
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{})
	if err != nil { fmt.Println("execute err:", err); return }
	fmt.Println("execute success, length:", buf.Len())
}
