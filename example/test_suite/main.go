package main

import (
	"fmt"
	"net/http"
	"time"

	engineigo "github.com/taogames/engine.igo"
	"github.com/taogames/engine.igo/message"
	"go.uber.org/zap"
)

func main() {
	conf := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := conf.Build()
	if err != nil {
		panic(err)
	}

	s := engineigo.NewServer(
		engineigo.WithPingInterval(time.Millisecond*300),
		engineigo.WithPingTimeout(time.Millisecond*200),
		engineigo.WithMaxPayload(1e6),
		engineigo.WithLogger(logger.Sugar()),
	)

	http.HandleFunc("/engine.io/", s.ServeHTTP)

	go func() {
		for {
			sess := <-s.Accept()
			fmt.Println("【Main】 Accepting session ", sess.ID())

			go func() {
				for {
					mt, bs, err := sess.ReadMessage()
					if err != nil {
						fmt.Println("【Main】 ReadMessage err ", err.Error())
						return
					}
					fmt.Println("【Main】 ReadMessage content", string(bs))

					if err := sess.WriteMessage(&message.Message{Type: mt, Data: bs}); err != nil {
						fmt.Println("【Main】 WriteMessage err ", err.Error())
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
