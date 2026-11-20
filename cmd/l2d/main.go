// l2d is a standalone existing-model companion CLI, independent of CoreServer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"local/l2dagent/l2d"
	"log"
	"os"
)

func main() {
	action := flag.String("action", "inspect", "inspect, launch, command")
	model := flag.String("model", "", "Cubism model3.json path")
	exe := flag.String("exe", "", "absolute ViewerEX exe path")
	endpoint := flag.String("endpoint", l2d.DefaultExAPI, "local WebSocket endpoint")
	message := flag.String("message", `{"msg":10000,"msgId":1}`, "official ExAPI JSON envelope")
	flag.Parse()
	var out any
	var e error
	switch *action {
	case "inspect":
		out, e = l2d.Inspect(*model)
	case "launch":
		e = l2d.Launch(*exe)
		out = map[string]string{"status": "process started; readiness unverified"}
	case "command":
		out, e = l2d.Command(context.Background(), *endpoint, json.RawMessage(*message))
	default:
		log.Fatal("unknown action")
	}
	if e != nil {
		log.Fatal(e)
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
