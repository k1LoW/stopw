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

	// Output:
	// {
	//   "id": "cbt0l1ev9mc36rrd56sg",
	//   "started_at": "2022-08-15T17:57:41.317696+09:00",
	//   "stopped_at": "2022-08-15T17:57:41.317702+09:00",
	//   "elapsed": 6333,
	//   "breakdown": [
	//     {
	//       "id": "sub span A",
	//       "started_at": "2022-08-15T17:57:41.317696+09:00",
	//       "stopped_at": "2022-08-15T17:57:41.317702+09:00",
	//       "elapsed": 5958,
	//       "breakdown": [
	//         {
	//           "id": "sub sub span a",
	//           "started_at": "2022-08-15T17:57:41.317696+09:00",
	//           "stopped_at": "2022-08-15T17:57:41.317701+09:00",
	//           "elapsed": 5375
	//         }
	//       ]
	//     },
	//     {
	//       "id": "sub span B",
	//       "started_at": "2022-08-15T17:57:41.317696+09:00",
	//       "stopped_at": "2022-08-15T17:57:41.317702+09:00",
	//       "elapsed": 6333
	//     }
	//   ]
	// }
}
```

