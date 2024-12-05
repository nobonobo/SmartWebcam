package main

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"strings"
	"syscall/js"

	"github.com/GoWebProd/uuid7"
	"github.com/chey/qr/code"

	"github.com/nobonobo/SmartWebcam/operator"
)

type M = map[string]interface{}
type S = []interface{}

var (
	document     = js.Global().Get("document")
	navigator    = js.Global().Get("navigator")
	location     = js.Global().Get("location")
	console      = js.Global().Get("console")
	mediaDevices = navigator.Get("mediaDevices")
)

const (
	index = `
	`
	viewer = `
	`
	camera = `
	<button id="activate"><h1>Camera ON</h1></button>
	<section style="display: grid">
		<label for="zoom" style="display: grid">
			<span>zoom:</span>
			<input type="range" id="zoom" />
		</label>
		<label for="focus" style="display: grid">
			<span>focus:</span>
			<input type="range" id="focus" />
		</label>
		<label for="expose" style="display: grid">
			<span>expose:</span>
			<input type="range" id="expose" />
		</label>
	</section>
	`
	failed = `
	<button id="restart"><h1>Restart</h1></button>
	`
)

func connect(stream js.Value, self, peer string) error {
	pc := js.Global().Get("RTCPeerConnection").New(M{
		"iceServers": S{
			M{
				"urls": "stun:stun.l.google.com:19302",
			},
		},
	})
	pc.Call("addEventListener", "icecandidate", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		candidate := args[0].Get("candidate")
		if candidate.IsNull() {
			data := pc.Get("localDescription").Get("sdp").String()
			console.Call("log", "icecandidate:done", data)
			go func() {
				if err := operator.Push(self, data); err != nil {
					console.Call("log", "push failed:", err.Error())
				}
			}()
			return nil
		}
		console.Call("log", "icecandidate:", candidate)
		return nil
	}))
	tracks := stream.Call("getTracks")
	for i := 0; i < tracks.Length(); i++ {
		pc.Call("addTrack", tracks.Index(i), stream)
	}
	offer, err := await(pc.Call("createOffer"))
	if err != nil {
		return fmt.Errorf("createOffer failed: %w", err)
	}
	console.Call("log", offer)
	pc.Call("setLocalDescription", offer)
	console.Call("log", "setLocalDescription:", offer)
	res, err := operator.Pull(peer, 3)
	if err != nil {
		console.Call("log", "pull failed:", err.Error())
		return err
	}
	await(pc.Call("setRemoteDescription", M{"type": "answer", "sdp": res}))
	return nil
}

var (
	track     js.Value
	zoomInput = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		track.Call("applyConstraints", M{
			"advanced": S{
				M{
					"zoom": args[0].Get("target").Get("value").Float(),
				},
			},
		})
		return nil
	})
	focusInput = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		track.Call("applyConstraints", M{
			"advanced": S{
				M{
					"focusMode":     "manual",
					"focusDistance": args[0].Get("target").Get("value").Float(),
				},
			},
		})
		return nil
	})
	exposeInput = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		track.Call("applyConstraints", M{
			"advanced": S{
				M{
					"exposureMode": "manual",
					"exposureTime": args[0].Get("target").Get("value").Float(),
				},
			},
		})
		return nil
	})
)

func show(view string) {
	view = strings.TrimLeft(view, "#/")
	switch view {
	case "failed":
		document.Get("body").Set("innerHTML", failed)
		document.Call("getElementById", "restart").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			location.Set("hash", "")
			return nil
		}))
	case "":
		document.Get("body").Set("innerHTML", index)
		params := js.Global().Get("URLSearchParams").New(location.Get("search"))
		selfObj := params.Call("get", "self")
		peerObj := params.Call("get", "peer")
		self, peer := selfObj.String(), peerObj.String()
		empty := selfObj.IsNull() || peerObj.IsNull()
		if empty {
			generator := uuid7.New()
			self, peer = generator.Next().String(), generator.Next().String()
		}
		u, _ := url.Parse(location.Get("origin").String() + location.Get("pathname").String())
		u.RawQuery = url.Values{
			"self": {self},
			"peer": {peer},
		}.Encode()
		if empty {
			location.Set("search", u.Query().Encode())
			return
		}
		u.Fragment = "camera"
		console.Call("log", "qr: ", u.String())
		qr, err := code.New(u.String(), code.Low)
		if err != nil {
			console.Call("log", "code.New failed:", err)
			return
		}
		buff := bytes.NewBuffer(nil)
		qr.PNG(buff)
		blob := js.Global().Get("Blob").New(
			S{Bytes2JS(buff.Bytes())},
			M{"type": "image/png"},
		)
		data := js.Global().Get("URL").Call("createObjectURL", blob)
		img := document.Call("createElement", "img")
		img.Set("src", data)
		document.Get("body").Call("appendChild", img)
		go func() {
			connected := false
			done := make(chan error, 1)
			sdp, err := operator.Pull(self, 3)
			if err != nil {
				console.Call("log", "pull failed:", err.Error())
				location.Set("hash", "#failed")
				return
			}
			console.Call("log", sdp)
			document.Get("body").Set("innerHTML", viewer)
			console.Call("log", self, peer)
			pc := js.Global().Get("RTCPeerConnection").New(M{
				"iceServers": S{
					M{
						"urls": "stun:stun.l.google.com:19302",
					},
				},
			})
			pc.Call("addEventListener", "connectionstatechange", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				console.Call("log", "pc:", pc)
				state := pc.Get("connectionState")
				console.Call("log", "connectionState:", state)
				switch state.String() {
				case "connected":
					connected = true
				case "disconnected":
					if connected {
						close(done)
					}
				case "failed":
					done <- fmt.Errorf("connection failed")
				}
				return nil
			}))
			pc.Call("addEventListener", "icecandidate", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				candidate := args[0].Get("candidate")
				if candidate.IsNull() {
					go func() {
						data := pc.Get("localDescription").Get("sdp").String()
						if err := operator.Push(peer, data); err != nil {
							log.Println("push failed:", err)
							done <- err
							return
						}
					}()
				}
				return nil
			}))
			pc.Call("addEventListener", "track", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				stream := args[0].Get("streams").Index(0)
				console.Call("log", "track:", stream)
				video := document.Call("createElement", "video")
				video.Set("srcObject", stream)
				video.Set("autoplay", true)
				video.Set("muted", true)
				video.Set("controls", true)
				document.Get("body").Call("appendChild", video)
				return nil
			}))
			await(pc.Call("setRemoteDescription", M{"type": "offer", "sdp": sdp}))
			answer, err := await(pc.Call("createAnswer"))
			if err != nil {
				log.Println("createAnswer failed:", err)
				done <- err
				return
			}
			await(pc.Call("setLocalDescription", answer))
			<-done
			location.Call("reload")
		}()
	case "camera":
		document.Get("body").Set("innerHTML", camera)
		params := js.Global().Get("URLSearchParams").New(location.Get("search"))
		self := params.Call("get", "self").String()
		peer := params.Call("get", "peer").String()
		document.Call("getElementById", "activate").Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			go func() {
				stream, err := await(mediaDevices.Call("getUserMedia", M{
					"audio": false,
					"video": M{"facingMode": "environment", "zoom": true},
				}))
				if err != nil {
					console.Call("log", "getUserMedia failed:", err)
					return
				}
				connect(stream, self, peer)
				document.Call("getElementById", "activate").Set("hidden", true)
				video := document.Call("createElement", "video")
				video.Set("srcObject", stream)
				video.Set("autoplay", true)
				video.Set("muted", true)
				//video.Set("controls", true)
				track = stream.Call("getVideoTracks").Index(0)
				capabilities := track.Call("getCapabilities")
				settings := track.Call("getSettings")
				zoom := document.Call("getElementById", "zoom")
				focus := document.Call("getElementById", "focus")
				expose := document.Call("getElementById", "expose")
				zoom.Set("min", capabilities.Get("zoom").Get("min").Float())
				zoom.Set("max", capabilities.Get("zoom").Get("max").Float())
				zoom.Set("step", capabilities.Get("zoom").Get("step").Int())
				zoom.Set("value", settings.Get("zoom").Float())
				focus.Set("min", capabilities.Get("focusDistance").Get("min").Float())
				focus.Set("max", capabilities.Get("focusDistance").Get("max").Float())
				focus.Set("step", capabilities.Get("focusDistance").Get("step").Int())
				focus.Set("value", settings.Get("focusDistance").Float())
				expose.Set("min", capabilities.Get("exposureTime").Get("min").Float())
				expose.Set("max", capabilities.Get("exposureTime").Get("max").Float())
				expose.Set("step", capabilities.Get("exposureTime").Get("step").Int())
				expose.Set("value", settings.Get("exposureTime").Float())
				zoom.Set("oninput", zoomInput)
				focus.Set("oninput", focusInput)
				expose.Set("oninput", exposeInput)
				document.Get("body").Call("appendChild", video)
			}()
			return nil
		}))
	}
}

func main() {
	show(location.Get("hash").String())
	js.Global().Call("addEventListener", "hashchange", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		println(location.Get("hash").String(), location.Get("search").String())
		show(location.Get("hash").String())
		return nil
	}))
	select {}
}
