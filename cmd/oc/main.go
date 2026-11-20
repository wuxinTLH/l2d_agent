// oc is a standalone local image module CLI, independent of CoreServer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"local/l2dagent/oc"
	"log"
	"os"
)

func main() {
	action := flag.String("action", "list", "list, import, generate, edit, split")
	root := flag.String("data", ".local/assets", "asset store")
	file := flag.String("file", "", "input image, or split region JSON file")
	id := flag.String("id", "", "source asset id")
	prompt := flag.String("prompt", "", "generation/edit prompt")
	base := flag.String("base", "https://api.openai.com/v1", "provider API base")
	model := flag.String("model", "gpt-image-1", "image model")
	size := flag.String("size", "1024x1024", "generation size")
	flag.Parse()
	s := oc.New(*root)
	p := oc.Provider{BaseURL: *base, Model: *model, Key: os.Getenv("OC_API_KEY")}
	var out any
	var e error
	switch *action {
	case "list":
		out, e = s.List()
	case "import":
		var b []byte
		b, e = os.ReadFile(*file)
		if e == nil {
			out, e = s.Import(b, "Imported OC")
		}
	case "generate":
		out, e = s.Generate(context.Background(), p, *prompt, "", *size)
	case "edit":
		var b []byte
		b, e = os.ReadFile(*file)
		if e == nil {
			out, e = s.Edit(context.Background(), p, b, *prompt)
		}
	case "split":
		var b []byte
		b, e = os.ReadFile(*file)
		var parts []oc.Part
		if e == nil {
			e = json.Unmarshal(b, &parts)
		}
		if e == nil {
			out, e = s.Split(*id, parts)
		}
	default:
		log.Fatal("unknown action")
	}
	if e != nil {
		log.Fatal(e)
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
