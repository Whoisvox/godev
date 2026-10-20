package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan int)
	defer close(ch)
	go func() {
		fmt.Println("set 1 into ch")
		ch <- 1
		fmt.Println("set 1 into ch,done")
	}()
	go func() {
		fmt.Println("set 2 into ch")
		fmt.Println("oops, i m stuck here for a while...")
		time.Sleep(10 * time.Second)
		ch <- 2
		fmt.Println("set 2 into ch,done")
	}()
	fmt.Println("im ready to take something out...")
	fmt.Printf("No.1 = %d\n", <-ch)
	fmt.Printf("No.2 = %d\n", <-ch)
	fmt.Printf("No.3 = %d", <-ch)
}
