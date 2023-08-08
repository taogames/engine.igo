package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	engineigo "github.com/taogames/engine.igo"
	"github.com/taogames/engine.igo/message"
)

func main() {
	s := engineigo.NewServer(
		engineigo.WithPingInterval(time.Millisecond*300),
		engineigo.WithPingTimeout(time.Millisecond*200),
		engineigo.WithMaxPayload(1e6),
	)

	http.HandleFunc("/engine.io/", s.ServeHTTP)

	go func() {
		for {
			sess := <-s.Accept()
			fmt.Println("Accepting session ", sess.ID())

			go func() {
				for {
					mt, pt, r, err := sess.NextReader()
					if err != nil {
						fmt.Println("【Main】NextReader err ", err.Error())
						return
					}
					fmt.Println("【Main】NextReader type: ", mt, pt)

					bs, err := io.ReadAll(r)
					if err != nil {
						fmt.Println("【Main】ReadAll err ", err.Error())
						return
					}
					r.Close()

					fmt.Println("【Main】NextReader content", string(bs))

					if err := sess.WriteMessage(&message.Message{Type: mt, Data: bs}); err != nil {
						fmt.Println("【Main】WriteMessage err ", err.Error())
						return
					}
				}
			}()
		}
	}()

	if err := http.ListenAndServe(":3000", nil); err != nil {
		panic(err)
	}

}
