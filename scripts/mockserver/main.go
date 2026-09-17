package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8089"
	}
	http.HandleFunc("/v1/chat/completions", openaiChat)
	http.HandleFunc("/anthropic/v1/messages", anthropicChat)
	http.HandleFunc("/vllm/v1/chat/completions", openaiChat)
	fmt.Println("mock listening on :" + addr)
	_ = http.ListenAndServe(":"+addr, nil)
}

func openaiChat(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	msg := "mock openai response"
	if body["stream"] == true {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, word := range strings.Fields(msg) {
			chunk := map[string]interface{}{
				"choices": []map[string]interface{}{{"delta": map[string]string{"content": word + " "}, "index": 0}},
			}
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id": "cmpl-mock", "object": "chat.completion",
		"choices": []map[string]interface{}{
			{"index": 0, "message": map[string]string{"role": "assistant", "content": msg}, "finish_reason": "stop"},
		},
	})
}

func anthropicChat(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	msg := "mock anthropic response"
	if body["stream"] == true {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, word := range strings.Fields(msg) {
			ev := map[string]interface{}{
				"type": "content_block_delta",
				"delta": map[string]string{"type": "text_delta", "text": word + " "},
			}
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: {\"type\":\"message_stop\"}\n\n")
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id": "msg-mock", "type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "text", "text": msg}},
	})
}
