package config

import "testing"

func TestPluginHelpers(t *testing.T) {
	c := &EMOSConfig{Plugin: &PluginInfo{Slug: "m20", Role: RoleRobot}}
	c.UpsertSensor(PluginInfo{Slug: "hik", Role: RoleSensor})
	c.UpsertSensor(PluginInfo{Slug: "livox", Role: RoleSensor})

	// Plugins() = robot + sensors.
	if got := len(c.Plugins()); got != 3 {
		t.Fatalf("Plugins() len = %d, want 3", got)
	}

	// UpsertSensor replaces by slug rather than appending a duplicate.
	c.UpsertSensor(PluginInfo{Slug: "hik", Role: RoleSensor, EntryPoint: "x:Y"})
	if got := len(c.SensorPlugins); got != 2 {
		t.Fatalf("after re-upsert, sensors = %d, want 2", got)
	}
	if p := c.FindPlugin("hik"); p == nil || p.EntryPoint != "x:Y" {
		t.Fatalf("FindPlugin(hik) = %+v, want the updated entry", p)
	}

	// FindPlugin reaches the robot and reports misses.
	if c.FindPlugin("m20") == nil {
		t.Fatal("FindPlugin(m20) = nil, want the robot")
	}
	if c.FindPlugin("nope") != nil {
		t.Fatal("FindPlugin(nope) != nil")
	}

	// RemovePlugin drops from the correct slot.
	if !c.RemovePlugin("livox") || len(c.SensorPlugins) != 1 {
		t.Fatalf("RemovePlugin(livox) failed; sensors = %v", c.SensorPlugins)
	}
	if !c.RemovePlugin("m20") || c.Plugin != nil {
		t.Fatal("RemovePlugin(m20) did not clear the robot")
	}
	if c.RemovePlugin("gone") {
		t.Fatal("RemovePlugin(gone) = true, want false")
	}
}
