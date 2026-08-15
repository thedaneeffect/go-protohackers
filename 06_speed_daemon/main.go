package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"slices"
	"sync"
	"time"
)

func main() {
	log.SetFlags(log.Lshortfile)
	addr := ":12345"
	listener, err := net.Listen("tcp", addr)

	if err != nil {
		panic(err)
	}

	log.Printf("Listening on %s...", addr)

	for {
		conn, err := listener.Accept()

		if err != nil {
			log.Print(err)
			continue
		}

		go func() {
			addr := conn.RemoteAddr().String()
			logger := log.New(log.Writer(), fmt.Sprintf("[%s]: ", addr), log.Lshortfile|log.Ldate|log.Ltime)

			defer conn.Close()

			logger.Printf("connected")
			defer logger.Printf("disconnected")

			if err := handle(conn, logger); err != nil {
				logger.Println(err)
				msg := err.Error()

				if len(msg) > 255 {
					msg = msg[:254]
				}

				buf := make([]byte, 0, 300)
				buf = append(buf, uint8(msg_error))
				buf = append(buf, uint8(len(msg)))
				buf = append(buf, msg...)
				_, _ = conn.Write(buf)
			}
		}()
	}
}

type msg uint8

const (
	msg_none           msg = 0x00
	msg_error          msg = 0x10
	msg_plate          msg = 0x20
	msg_ticket         msg = 0x21
	msg_want_heartbeat msg = 0x40
	msg_heartbeat      msg = 0x41
	msg_am_camera      msg = 0x80
	msg_am_dispatcher  msg = 0x81
)

func (m msg) String() string {
	switch m {
	case msg_none:
		return "none"
	case msg_error:
		return "error"
	case msg_plate:
		return "plate"
	case msg_ticket:
		return "ticket"
	case msg_want_heartbeat:
		return "want_heartbeat"
	case msg_heartbeat:
		return "heartbeat"
	case msg_am_camera:
		return "am_camera"
	case msg_am_dispatcher:
		return "am_dispatcher"
	}
	return "unknown"
}

type dispatcher_t struct {
	roads []uint16
}

type camera_t struct {
	road  uint16
	mile  uint16
	limit uint16
}

type observation_t struct {
	mile           uint16
	timestamp_unix uint32
}

type plate_t struct {
	observations map[uint16][]observation_t
	ticketed     map[uint32]bool
	mu           sync.Mutex
}

type ticket_t struct {
	plate      string
	road       uint16
	mile1      uint16
	timestamp1 uint32
	mile2      uint16
	timestamp2 uint32
	velocity   uint16
}

func (t ticket_t) marshal() []byte {
	buf := make([]byte, 0, 1+1+len(t.plate)+2+4+2+4+2)
	buf = append(buf, uint8(msg_ticket))
	buf = append(buf, uint8(len(t.plate)))
	buf = append(buf, t.plate...)
	buf = binary.BigEndian.AppendUint16(buf, t.road)
	buf = binary.BigEndian.AppendUint16(buf, t.mile1)
	buf = binary.BigEndian.AppendUint32(buf, t.timestamp1)
	buf = binary.BigEndian.AppendUint16(buf, t.mile2)
	buf = binary.BigEndian.AppendUint32(buf, t.timestamp2)
	buf = binary.BigEndian.AppendUint16(buf, t.velocity)
	return buf
}

var (
	mu          sync.Mutex
	pending     = make(map[uint16][]ticket_t)
	dispatchers = make(map[net.Conn]*dispatcher_t)
	plates      sync.Map
)

func handle(conn net.Conn, logger *log.Logger) error {
	current_message := msg_none
	has_heartbeat := false

	var state any

	defer func() {
		switch state.(type) {
		case *dispatcher_t:
			mu.Lock()
			defer mu.Unlock()
			delete(dispatchers, conn)
		}
	}()

	var buf [256 * 2]byte

	read := func(n int, purpose string) ([]byte, error) {
		if _, err := io.ReadFull(conn, buf[:n]); err != nil {
			return nil, fmt.Errorf("%s: read: %w", purpose, err)
		}
		return buf[:n], nil
	}

	read_u8 := func(purpose string) (uint8, error) {
		data, err := read(1, purpose)
		if err != nil {
			return 0, err
		}
		return uint8(data[0]), nil
	}

	read_u32 := func(purpose string) (uint32, error) {
		data, err := read(4, purpose)
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint32(data), nil
	}

	for {
		switch current_message {
		case msg_none:
			data, err := read(1, "header")
			if err != nil {
				return err
			}
			current_message = msg(data[0])
			logger.Printf("next message: %s (0x%X)", current_message, uint32(current_message))

		case msg_plate:
			camera, ok := state.(*camera_t)

			if !ok {
				return fmt.Errorf("expected plate from camera, got %T", camera)
			}

			plate_length, err := read_u8("plate length")
			if err != nil {
				return err
			}

			plate_data, err := read(int(plate_length), "plate string")
			if err != nil {
				return err
			}
			id := string(plate_data)

			timestamp_unix, err := read_u32("plate timestamp")
			if err != nil {
				return err
			}

			logger.Println("plate", id, time.Unix(int64(timestamp_unix), 0))

			func() {
				plate := &plate_t{
					observations: make(map[uint16][]observation_t),
					ticketed:     make(map[uint32]bool),
				}

				value, loaded := plates.LoadOrStore(id, plate)

				observation := observation_t{
					mile:           camera.mile,
					timestamp_unix: timestamp_unix,
				}

				if !loaded {
					plate.observations[camera.road] = []observation_t{observation}
					return
				}

				plate = value.(*plate_t)

				plate.mu.Lock()
				defer plate.mu.Unlock()

				observations := plate.observations[camera.road]

				for _, other := range observations {
					var mile1 uint16
					var mile2 uint16
					var timestamp1_unix uint32
					var timestamp2_unix uint32

					if other.timestamp_unix < timestamp_unix {
						timestamp1_unix = other.timestamp_unix
						mile1 = other.mile
						timestamp2_unix = timestamp_unix
						mile2 = camera.mile
					} else {
						timestamp2_unix = other.timestamp_unix
						mile2 = other.mile
						timestamp1_unix = timestamp_unix
						mile1 = camera.mile
					}

					timestamp1 := time.Unix(int64(timestamp1_unix), 0)
					timestamp2 := time.Unix(int64(timestamp2_unix), 0)

					distance := math.Abs(float64(mile2) - float64(mile1))
					elapsed := timestamp2.Sub(timestamp1)

					if elapsed == 0 {
						continue
					}

					velocity := distance / elapsed.Hours()

					if math.Round(velocity) > float64(camera.limit) {
						day1 := (timestamp1_unix / 86400)
						day2 := (timestamp2_unix / 86400)

						clear := true
						for day := day1; day <= day2; day++ {
							if plate.ticketed[day] {
								clear = false
								break
							}
						}

						if clear {
							for day := day1; day <= day2; day++ {
								plate.ticketed[day] = true
							}
							deliver(ticket_t{
								plate:      id,
								road:       camera.road,
								mile1:      mile1,
								timestamp1: timestamp1_unix,
								mile2:      mile2,
								timestamp2: timestamp2_unix,
								velocity:   uint16(velocity * 100),
							})
						}
					}
				}

				plate.observations[camera.road] = append(plate.observations[camera.road], observation)
			}()

			current_message = msg_none

		case msg_want_heartbeat:
			if has_heartbeat {
				return fmt.Errorf("dup heartbeat")
			}
			deciseconds, err := read_u32("deciseconds")
			if err != nil {
				return err
			}
			if deciseconds == 0 {
				current_message = msg_none
				continue
			}

			duration := 100 * time.Millisecond // 100ms = 1ds
			duration *= time.Duration(deciseconds)

			go func(duration time.Duration) {
				ticker := time.NewTicker(duration)

				heartbeat := [1]byte{byte(msg_heartbeat)}
				for {
					<-ticker.C
					if _, err := conn.Write(heartbeat[:]); err != nil {
						logger.Println(fmt.Errorf("write heartbeat: %w", err))
						return
					}
				}
			}(duration)

			logger.Printf("want heartbeat: %s", duration)

			has_heartbeat = true
			current_message = msg_none

		case msg_am_camera:
			if state != nil {
				return fmt.Errorf("role already assigned: %T", state)
			}

			if _, err := io.ReadFull(conn, buf[:6]); err != nil {
				return fmt.Errorf("camera read: %w", err)
			}

			camera := &camera_t{}
			camera.road = binary.BigEndian.Uint16(buf[0:])
			camera.mile = binary.BigEndian.Uint16(buf[2:])
			camera.limit = binary.BigEndian.Uint16(buf[4:])
			state = camera

			logger.Printf("is cam: %+v", camera)

			current_message = msg_none

		case msg_am_dispatcher:
			if state != nil {
				return fmt.Errorf("role already assigned: %T", state)
			}

			count, err := read_u8("road count")
			if err != nil {
				return err
			}

			roads, err := read(int(count)*2, "road data")
			if err != nil {
				return err
			}

			dispatcher := &dispatcher_t{}
			dispatcher.roads = make([]uint16, count)
			for i := range dispatcher.roads {
				dispatcher.roads[i] = binary.BigEndian.Uint16(roads[i*2:])
			}
			state = dispatcher

			mu.Lock()
			dispatchers[conn] = dispatcher
			for _, road := range dispatcher.roads {
				for _, t := range pending[road] {
					_, _ = conn.Write(t.marshal())
				}
				delete(pending, road)
			}
			mu.Unlock()

			logger.Printf("is dispatcher: %+v", dispatcher.roads)

			current_message = msg_none

		default:
			return fmt.Errorf("unknown message: 0x%X", current_message)
		}
	}
}

func deliver(t ticket_t) {
	mu.Lock()
	defer mu.Unlock()
	for conn, d := range dispatchers {
		if slices.Contains(d.roads, t.road) {
			if _, err := conn.Write(t.marshal()); err != nil {
				delete(dispatchers, conn) // dead conn
				continue
			}
			return
		}
	}
	pending[t.road] = append(pending[t.road], t)
}
