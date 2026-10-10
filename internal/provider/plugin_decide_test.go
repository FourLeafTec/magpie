package provider

import (
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/plugin"
)

// A plugin provider that serves a decision API: the plugin says so on its
// config hook (plugin.Provider.Decide), and magpie asks it through the same
// base as the provider's conversations, at /systemone. Without the flag the
// provider has no decision API at all, which is what a plugin that serves
// only conversations wants.
func TestPluginProviderDecide(t *testing.T) {
	claudeHome(t)
	pp := plugin.Provider{ID: "fakeco", Name: "fakeco", NPM: "@ai-sdk/openai-compatible",
		Decide: true, Models: []plugin.Model{{ID: "jev-1.13-free", Decides: true, NPM: "@ai-sdk/openai-compatible"}}}
	p := pluginProvider(pp, pluginLogin{})

	if !p.Decides() {
		t.Fatal("a plugin provider that declares decide has no decision API")
	}
	// the same base as its conversations, through the plugin's own fetch:
	// pluginFetch refuses anywhere but the loader's host
	if want := pluginBase + "fakeco/v1"; p.Decide != want {
		t.Fatalf("Decide = %q, want %q", p.Decide, want)
	}
	if p.Decide != p.Chat {
		t.Fatalf("decide %q and chat %q are not the same base", p.Decide, p.Chat)
	}
	if !p.DecidesModel("jev-1.13-free") {
		t.Fatal("a plugin's marked Jev model is not seen as a decision model")
	}
	if p.DecideOnly() {
		t.Fatal("a plugin that serves conversations too is not decide-only")
	}
	// and it is offered as a classifier, not left to the name rule
	var ids []string
	for _, e := range p.DecisionModels() {
		ids = append(ids, e.ID)
	}
	if len(ids) != 1 || ids[0] != "jev-1.13-free" {
		t.Fatalf("marked decisions = %v, want [jev-1.13-free]", ids)
	}
	if p.Jev() != "jev-1.13-free" {
		t.Fatalf("Jev() = %q, want the model the plugin marked", p.Jev())
	}
}

// A decision model a plugin serves that isn't called Jev: the name rule
// can't see it, so the plugin's mark is the only way. This is the case the
// marking exists for (Cloudflare's Clef, a vendor's own name).
func TestPluginDecisionModelNamedOtherwise(t *testing.T) {
	claudeHome(t)
	pp := plugin.Provider{ID: "fakeco", Name: "fakeco", Decide: true, Models: []plugin.Model{
		{ID: "clef", Name: "Clef", Decides: true},
		{ID: "clef-flash", Name: "Clef Flash", Decides: true},
		{ID: "chat-model", Name: "Chat"},
	}}
	p := pluginProvider(pp, pluginLogin{})

	// the name rule alone would miss these two
	if jevID("clef") || CloudflareClef("clef") && p.DecideVia() == ViaCloudflare {
		t.Fatal("this test no longer exercises the marking: the name rule sees it")
	}
	if !p.DecidesModel("clef") || !p.DecidesModel("clef-flash") {
		t.Fatal("a plugin's marked decision model is not seen as one")
	}
	if p.DecidesModel("chat-model") {
		t.Fatal("a model the plugin did not mark is taken as a decision model")
	}
	if got := p.Jev(); got != "" {
		// neither marked model is named Jev: nothing to ask by default
		t.Fatalf("Jev() = %q, want none for a plugin whose decisions aren't Jev", got)
	}
	// still offered to a routing group's picker
	var ids []string
	for _, e := range p.DecisionModels() {
		ids = append(ids, e.ID)
	}
	if len(ids) != 2 || ids[0] != "clef" || ids[1] != "clef-flash" {
		t.Fatalf("marked decisions = %v, want [clef clef-flash]", ids)
	}
	// and not among the models an agent chats with: isDecision is the gate
	// the conversation catalog skips them by (buildEntries, models.go)
	byID := map[string]catalog.Model{}
	for _, m := range p.Available() {
		byID[m.ID] = m
	}
	if !p.isDecision(byID["clef"]) || !p.isDecision(byID["clef-flash"]) {
		t.Fatal("a marked decision model is taken as one to chat with")
	}
	if p.isDecision(byID["chat-model"]) {
		t.Fatal("the plugin's chat model is taken as a decision model")
	}
}

// A routing group's classifier names the model as "provider/model", which
// resolves through resolveDecideModel: a marked model the name rule can't
// see must resolve there too, or a Clef could never be a classifier.
func TestPluginDecisionModelResolves(t *testing.T) {
	claudeHome(t)
	pp := plugin.Provider{ID: "fakeco", Name: "fakeco", Decide: true, Models: []plugin.Model{
		{ID: "clef", Decides: true},
		{ID: "chat-model"},
	}}
	p := pluginProvider(pp, pluginLogin{})
	if got, ok := resolveDecideModel(p, "clef"); !ok || got != "clef" {
		t.Fatalf("resolveDecideModel(clef) = %q, %v; want clef, true", got, ok)
	}
	if _, ok := resolveDecideModel(p, "chat-model"); ok {
		t.Fatal("a model the plugin did not mark resolves as a decider")
	}
}

// A plugin provider without the flag keeps exactly what it had: its models
// are conversation models, and it has no decision API.
func TestPluginProviderWithoutDecide(t *testing.T) {
	claudeHome(t)
	pp := plugin.Provider{ID: "fakeco", Name: "fakeco", NPM: "@ai-sdk/openai-compatible",
		Models: []plugin.Model{{ID: "jev-1.13-free", Decides: true, NPM: "@ai-sdk/openai-compatible"}}}
	p := pluginProvider(pp, pluginLogin{})

	if p.Decides() {
		t.Fatalf("a plugin provider that says nothing has a decision API: %q", p.Decide)
	}
	if p.DecidesModel("jev-1.13-free") {
		t.Fatal("a plugin provider with no decision API decides its model anyway")
	}
	if len(p.DecisionModels()) != 0 {
		t.Fatalf("a provider with no decision API lists decisions: %v", p.DecisionModels())
	}
}

// The URL magpie derives is the one pluginFetch can send: the same host as
// the loader's base. A URL anywhere else would fail with the host check
// rather than reach the plugin.
func TestPluginDecideURLGoesThroughThePlugin(t *testing.T) {
	claudeHome(t)
	pp := plugin.Provider{ID: "fakeco", Name: "fakeco", Decide: true}
	p := pluginProvider(pp, pluginLogin{})
	if p.Decide != pluginBase+"fakeco/v1" {
		t.Fatalf("Decide = %q, want the plugin's own base", p.Decide)
	}
}
