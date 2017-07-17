package ws

import (
	"babelweb2/parser"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	delete = iota
	update = iota
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var Db dataBase

//Message messages to send to the client via websocket
type Message map[string]interface{}

type dataBase struct {
	sync.Mutex
	Bd parser.BabelDesc
}

//MCUpdates multicast updates sent by the routine comminicating with the routers
func MCUpdates(updates chan parser.BabelUpdate, g *Listenergroupe,
	wg *sync.WaitGroup) {
	wg.Add(1)
	for {
		update, quit := <-updates
		if !quit {
			log.Println("closing all channels")
			g.Iter(func(l *Listener) {
				close(l.conduct)
			})
			wg.Done()
			return
		}
		if !(Db.Bd.CheckUpdate(update)) {
			continue
		}
		Db.Lock()
		err := Db.Bd.Update(update)
		if err != nil {
			log.Println(err)
		}
		Db.Unlock()
		t := update.ToS()
		g.Iter(func(l *Listener) {
			l.conduct <- t
		})
		//TODO unlock()
	}
}

//GetMess gets messages sent by the client and redirect them to the mess chanel
func GetMess(conn *websocket.Conn, mess chan []byte) {
	_, message, err := conn.ReadMessage()
	if err != nil {
		close(mess)
		log.Println(err)
		return
	}
	mess <- message

}

//HandleMessage handle messages receved from the client
func HandleMessage(mess []byte) {
	//TODO gere les message du client
	return
}

//Handler manage the websockets
func Handler(l *Listenergroupe) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Could not create the socket.", err)
			return
		}

		Db.Lock()
		Db.Bd.Iter(func(bu parser.BabelUpdate) error {
			sbu := bu.ToS()
			err := conn.WriteJSON(sbu)
			if err != nil {
				log.Println(err)
			}
			return err
		})
		Db.Unlock()

		log.Println("New connection to a websocket")
		updates := NewListener()
		l.Push(updates)
		defer l.Flush(updates)
		mess := make(chan []byte, ChanelSize)
		go GetMess(conn, mess)
		for {
			//we wait for a new message from the client or from our channel
			select {
			case lastUp := <-updates.conduct: //we got a new update on the channel

				log.Println("sending:\n", lastUp)

				err := conn.WriteJSON(lastUp)
				if err != nil {
					log.Println(err)
				}

			case clientMessage, q := <-mess: //we got a message from the client
				if q == false {
					return
				}

				log.Println(clientMessage)

				HandleMessage(clientMessage)

			}
		}
	}
	return http.HandlerFunc(fn)
}
