package debridlink

import (
	"testing"

	json "github.com/bytedance/sonic"
)

func FuzzAvailableResponseDecode(f *testing.F) {
	f.Add(`{"success":true,"value":{"hash":{"v":{"name":"file","hashString":"hash","files":[{"name":"a","size":1}]}}}}`)
	f.Add(`{"success":true,"value":null}`)
	f.Add(`{"success":false,"value":{}}`)
	f.Add(`[]`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, payload string) {
		var out AvailableResponse
		_ = json.Unmarshal([]byte(payload), &out)
	})
}

func FuzzTorrentInfoDecode(f *testing.F) {
	f.Add(`{"success":true,"value":[{"id":"1","name":"n","hashString":"h","status":1,"files":[{"id":"f1","name":"a","downloadUrl":"u","size":1,"downloadPercent":0}]}]}`)
	f.Add(`{"success":true,"value":[]}`)
	f.Add(`{"success":true,"value":null}`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, payload string) {
		var out torrentInfo
		_ = json.Unmarshal([]byte(payload), &out)
	})
}
