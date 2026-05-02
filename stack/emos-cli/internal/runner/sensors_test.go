package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// recipeFixture is a hand-rolled recipe file shaped like a real one. The AST
// walker has to find both bare `Topic(...)` calls and `module.Topic(...)`
// calls, with name + msg_type as keyword args. Anything else (positional
// args, missing kwargs) must be ignored without crashing.
const recipeFixture = `
import sys
from automatika import Topic

# Sensor
camera = Topic(name="/camera/image", msg_type="Image")

# Non-sensor
control = Topic(name="cmd/vel", msg_type="Twist")

# Attribute-style call
audio = sys.module.Topic(name="/mic/audio", msg_type="Audio")

# Should be skipped — positional args
ignored1 = Topic("/positional/skip", "Image")

# Should be skipped — missing msg_type
ignored2 = Topic(name="/halffilled")
`

func TestExtractTopicsAndClassify(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not on PATH; ExtractTopics requires it")
	}

	path := filepath.Join(t.TempDir(), "recipe.py")
	if err := os.WriteFile(path, []byte(recipeFixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	topics, err := ExtractTopics(path)
	if err != nil {
		t.Fatalf("ExtractTopics: %v", err)
	}
	if len(topics) != 3 {
		t.Fatalf("got %d topics, want 3: %+v", len(topics), topics)
	}

	// Sort for stable assertions; AST walk order is an implementation detail.
	sort.Slice(topics, func(i, j int) bool { return topics[i].Name < topics[j].Name })

	want := []ExtractedTopic{
		{Name: "/camera/image", MsgType: "Image", IsSensor: true},
		{Name: "/mic/audio", MsgType: "Audio", IsSensor: true},
		{Name: "cmd/vel", MsgType: "Twist", IsSensor: false},
	}
	for i, w := range want {
		if topics[i] != w {
			t.Errorf("topic[%d] = %+v, want %+v", i, topics[i], w)
		}
	}
}

func TestSensorAndOtherTopics(t *testing.T) {
	in := []ExtractedTopic{
		{Name: "/img", MsgType: "Image", IsSensor: true},
		{Name: "/cmd", MsgType: "Twist", IsSensor: false},
		{Name: "/imu", MsgType: "Imu", IsSensor: true},
	}
	sensors := SensorTopics(in)
	if len(sensors) != 2 {
		t.Fatalf("SensorTopics len = %d, want 2", len(sensors))
	}
	other := OtherTopics(in)
	if len(other) != 1 || other[0].Name != "/cmd" {
		t.Fatalf("OtherTopics = %+v", other)
	}
}
