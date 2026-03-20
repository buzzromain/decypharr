package arr

import (
	"testing"

	json "github.com/bytedance/sonic"
)

func FuzzWebhookPayloadParsing(f *testing.F) {
	f.Add(`{"eventType":"Download","instanceName":"sonarr","downloadId":"abc","episodeFile":{"path":"/tv/a.mkv"}}`)
	f.Add(`{"eventType":"Rename","renamedEpisodeFiles":[{"previousPath":"/old.mkv","path":"/new.mkv"}]}`)
	f.Add(`{"eventType":"MovieDelete","movie":{"id":12,"folderPath":"/movies/x"}}`)
	f.Add(``)
	f.Add(`{`)
	f.Add(`[]`)

	f.Fuzz(func(t *testing.T, payload string) {
		var p WebhookPayload
		_ = json.Unmarshal([]byte(payload), &p)
		_ = p.ManagedPaths()
		_ = p.PreviousPaths()
	})
}
