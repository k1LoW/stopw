package stopw_test

import (
	"encoding/json"
	"fmt"

	"github.com/k1LoW/stopw"
)

func Example() {
	stopw.Start()
	stopw.Start("sub span A")
	// do something for `sub span A`
	stopw.Start("sub span B")
	// do something for `sub span B`
	stopw.Start("sub span A", "sub sub span a")
	// do something for `sub sub span a`
	stopw.Stop("sub span A", "sub sub span a")
	stopw.Stop("sub span span A")
	stopw.Stop()

	r := stopw.Result()
	b, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(b))
}
