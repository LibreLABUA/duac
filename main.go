package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/cheggaaa/pb"
	"github.com/howeyc/gopass"
)

var (
	errors []error
	pool   = pb.NewPool()

	output = flag.String("o", "./files/", "Output directory")
)

func init() {
	flag.Parse()

	os.Args = append(os.Args[:1], flag.Args()...)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("%s [options] <username>@alu.ua.es\n", os.Args[0])
		os.Exit(0)
	}

	fmt.Printf("Password: ")
	pass, err := gopass.GetPasswd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if !strings.Contains(os.Args[1], "@alu") {
		os.Args[1] += "@alu.ua.es"
	}

	client, cookies, err := login(
		os.Args[1], b2s(pass),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	items := getFolders(client, cookies)
	for n := range items {
		p := pb.New(0)
		pool.Add(p)
		wg.Add(1)
		go func(i int, p *pb.ProgressBar) {
			do(p, client, cookies, items[i])
			wg.Done()
		}(n, p)
	}
	pool.Start()
	wg.Wait()
	pool.Stop()

	if len(errors) == 0 {
		fmt.Println("No errors reported")
		return
	}

	fmt.Println("Reported errors:")
	for _, err := range errors {
		fmt.Printf("\t- %s\n", err)
	}
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
