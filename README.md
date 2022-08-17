# stopw [![Go Reference](https://pkg.go.dev/badge/github.com/k1LoW/stopw.svg)](https://pkg.go.dev/github.com/k1LoW/stopw)

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
	//   "started_at": "2009-11-10T23:00:00.436022+09:00",
	//   "stopped_at": "2009-11-10T23:00:00.436883+09:00",
	//   "elapsed": 860375,
	//   "breakdown": [
	//     {
	//       "id": "sub span A",
	//       "started_at": "2009-11-10T23:00:00.436153+09:00",
	//       "stopped_at": "2009-11-10T23:00:00.436594+09:00",
	//       "elapsed": 441292,
	//       "breakdown": [
	//         {
	//           "id": "sub sub span a",
	//           "started_at": "2009-11-10T23:00:00.436449+09:00",
	//           "stopped_at": "2009-11-10T23:00:00.436594+09:00",
	//           "elapsed": 145500
	//         }
	//       ]
	//     },
	//     {
	//       "id": "sub span B",
	//       "started_at": "2009-11-10T23:00:00.436303+09:00",
	//       "stopped_at": "2009-11-10T23:00:00.436883+09:00",
	//       "elapsed": 580083
	//     }
	//   ]
	// }
}
```

### Measure elapsed time of block

``` go
func () {
	defer stopw.Start().Stop()
	// do something
}()
r := stopw.Result()
[...]
```

### Measure separately

``` go
a := stopw.New()
b := stopw.New()

a.Start()
// do something for a
a.Stop()

b.Start()
// do something for b
b.Stop()

ra := a.Result()
rb := b.Result()
```

