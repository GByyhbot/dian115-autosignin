//go:build wasip1 && wasm

// Reactor ABI follows the DIAN115 public WASM runtime v1 (TinyGo build).
// JSON 使用 pluginjson 做字段级读写，不链接 encoding/json（宿主构建要求）。
package main

import (
	"dian115-autosignin/app"
	"dian115-autosignin/host"
	"dian115-autosignin/pluginjson"
	"errors"
	"runtime"
	"time"
	"unsafe"
)

const frameLimit = 16 << 20

//go:wasmimport dian115 host_call
func hostCall(ptr, length uint32) uint32

//go:wasmimport dian115 host_read
func hostRead(ptr, length uint32) uint32

type broker struct{}

func (broker) Call(method string, params map[string]any) (pluginjson.Raw, error) {
	request, err := pluginjson.Encode(map[string]any{"method": method, "params": params})
	if err != nil {
		return nil, err
	}
	size := hostCall(uint32(uintptr(unsafe.Pointer(&request[0]))), uint32(len(request)))
	runtime.KeepAlive(request)
	if size == 0 || size > 2<<20 {
		return nil, errors.New("host response exceeds plugin budget")
	}
	response := make([]byte, size)
	if hostRead(uint32(uintptr(unsafe.Pointer(&response[0]))), size) != size {
		return nil, errors.New("host read failed")
	}
	if text := pluginjson.Text(pluginjson.Field(response, "error")); text != "" {
		return nil, errors.New("host RPC failed: " + text)
	}
	return pluginjson.Field(response, "result"), nil
}

var inputBuffer, outputBuffer []byte
var hostClient = host.Client{Caller: broker{}}
var service = app.Service{Store: hostClient, Broker: hostClient, HostClient: &hostClient}

//go:wasmexport dian115_alloc
func allocate(size uint32) uint32 {
	if size == 0 || size > frameLimit {
		panic("invalid allocation")
	}
	inputBuffer = make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&inputBuffer[0])))
}

//go:wasmexport dian115_handle
func handle(ptr, length uint32) uint64 {
	if len(inputBuffer) == 0 || ptr != uint32(uintptr(unsafe.Pointer(&inputBuffer[0]))) || length == 0 || int(length) > len(inputBuffer) {
		panic("invalid invocation buffer")
	}
	result, err := dispatch(inputBuffer[:length])
	envelope := map[string]any{"result": result}
	if err != nil {
		envelope = map[string]any{"error": map[string]any{"code": -32602, "message": err.Error()}}
	}
	outputBuffer, _ = pluginjson.Encode(envelope)
	if len(outputBuffer) > 256<<10 {
		outputBuffer = []byte(`{"error":{"code":-32603,"message":"response exceeds 256 KiB"}}`)
	}
	return uint64(uintptr(unsafe.Pointer(&outputBuffer[0])))<<32 | uint64(len(outputBuffer))
}

func dispatch(raw []byte) (any, error) {
	method := pluginjson.Text(pluginjson.Field(raw, "method"))
	params := pluginjson.Field(raw, "params")
	switch method {
	case "runtime.initialize":
		if pluginjson.Text(pluginjson.Field(params, "protocol")) != "dian115:wasm@1" {
			return nil, errors.New("unsupported protocol")
		}
		// 启动时探测宿主能力，供 UI 与凭据入口决策；失败不阻塞初始化。
		if features, err := hostClient.Capabilities(); err == nil {
			service.Features = features
		}
		return map[string]any{"ready": true, "protocol": "dian115:wasm@1"}, nil
	case "runtime.shutdown":
		return map[string]any{"stopping": true}, nil
	case "runtime.invoke":
		envelope := pluginjson.Field(params, "envelope")
		op := pluginjson.Text(pluginjson.Field(envelope, "op"))
		invocationID := pluginjson.Text(pluginjson.Field(envelope, "invocation_id"))
		payload := pluginjson.Field(envelope, "payload")
		if op == "resident" {
			for {
				_ = service.Tick(time.Now())
				time.Sleep(time.Minute)
			}
		}
		return service.Invoke(op, invocationID, payload)
	default:
		return nil, errors.New("unsupported method")
	}
}

func main() {}
