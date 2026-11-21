// l2d inspects existing Cubism exports and discovers or launches Cubism Editor.
package main

import (
	"encoding/json"
	"flag"
	"local/l2dagent/l2d"
	"log"
	"os"
)

func main() {
	action := flag.String("action", "inspect", "inspect, cubism-discover, cubism-launch")
	model := flag.String("model", "", "Cubism model3.json path")
	exe := flag.String("exe", "", "absolute CubismEditor*.exe editor path")
	flag.Parse()
	var out any
	var e error
	switch *action {
	case "inspect":
		out, e = l2d.Inspect(*model)
	case "cubism-discover":
		out = map[string]any{"candidates": l2d.DiscoverCubism()}
	case "cubism-launch":
		e = l2d.LaunchCubism(*exe)
		out = map[string]string{"status": "process started; editor readiness, license and project state unverified"}
	default:
		log.Fatal("unknown action")
	}
	if e != nil {
		log.Fatal(e)
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
