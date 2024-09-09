package main

import "syscall/js"

// Bytes2JS convert from byte slice for Go to Uint8Array for JS.
func Bytes2JS(b []byte) js.Value {
	res := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(res, b)
	return res
}

func await(promise js.Value) (js.Value, error) {
	done := make(chan struct{})
	var result js.Value
	var err error
	success := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		result = args[0]
		close(done)
		return nil
	})
	defer success.Release()
	failed := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		err = js.Error{Value: args[0]}
		close(done)
		return nil
	})
	defer failed.Release()
	promise.Call("then", success, failed)
	<-done
	return result, err
}
