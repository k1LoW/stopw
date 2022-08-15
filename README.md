# stopw

A stopwatch library in Go for nested time measurement.

## Usage

``` go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/k1LoW/stopw"
)

func main() {
	stopw.Start()
	stopw.Start("sub span A")
	// do something for `sub span A`
	stopw.Start("sub span B")
	// do something for `sub span A` or `sub span B`
	stopw.Start("sub span A", "sub sub span a")
	// do something for `sub span A` or `sub span B` or `sub sub span a`
	stopw.Stop("sub span A", "sub sub span a")
	// do something for `sub span A` or `sub span B`
	stopw.Stop("sub span span A")
	// do something for `sub span B`
	stopw.Stop()

	r := stopw.Result()
	b, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(b))

	// Output:
	// {
	//   "id": "cbt1386v9mc80ofooblg",
	//   "started_at": "2022-08-15T18:28:00.436022+09:00",
	//   "stopped_at": "2022-08-15T18:28:00.436883+09:00",
	//   "elapsed": 860375,
	//   "breakdown": [
	//     {
	//       "id": "sub span A",
	//       "started_at": "2022-08-15T18:28:00.436153+09:00",
	//       "stopped_at": "2022-08-15T18:28:00.436594+09:00",
	//       "elapsed": 441292,
	//       "breakdown": [
	//         {
	//           "id": "sub sub span a",
	//           "started_at": "2022-08-15T18:28:00.436449+09:00",
	//           "stopped_at": "2022-08-15T18:28:00.436594+09:00",
	//           "elapsed": 145500
	//         }
	//       ]
	//     },
	//     {
	//       "id": "sub span B",
	//       "started_at": "2022-08-15T18:28:00.436303+09:00",
	//       "stopped_at": "2022-08-15T18:28:00.436883+09:00",
	//       "elapsed": 580083
	//     }
	//   ]
	// }
}
```

