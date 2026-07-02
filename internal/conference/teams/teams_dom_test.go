package teams_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/odsod/recorder/internal/conference/teams"
)

// TestSnapshotJS evaluates the snapshot JavaScript expression against a fixture
// DOM that replicates both Teams nesting patterns (name at parent vs grandparent).
// This catches regressions where Teams changes their DOM hierarchy.
func TestSnapshotJS(t *testing.T) {
	requireNode(t)

	p := teams.New()
	js := p.SnapshotExpression()

	result := evalWithFixture(t, js, teamsFixtureHTML)

	snapshots, err := p.ParseSnapshot(result)
	if err != nil {
		t.Fatalf("ParseSnapshot: %v", err)
	}

	want := map[string]bool{
		"Alice Direct":  true, // name on parent (depth 1)
		"Bob Nested":    true, // name on grandparent (depth 2)
		"Carol Wrapped": true, // name on grandparent, with video-item-container in between
	}

	if len(snapshots) != len(want) {
		t.Fatalf("expected %d participants, got %d: %+v", len(want), len(snapshots), snapshots)
	}
	for _, s := range snapshots {
		if !want[s.Name] {
			t.Errorf("unexpected participant: %q", s.Name)
		}
		if len(s.Classes) == 0 {
			t.Errorf("participant %q has no classes", s.Name)
		}
	}
}

// TestPollJS evaluates the poll JavaScript expression and verifies it correctly
// reports speaking state across both nesting patterns.
func TestPollJS(t *testing.T) {
	requireNode(t)

	p := teams.New()
	pollJS, err := p.PollExpression("speaking-active")
	if err != nil {
		t.Fatal(err)
	}

	result := evalWithFixture(t, pollJS, teamsFixtureHTML)

	participants, err := p.ParsePoll(result)
	if err != nil {
		t.Fatalf("ParsePoll: %v", err)
	}

	want := map[string]bool{
		"Alice Direct":  true,  // has speaking-active class
		"Bob Nested":    false, // does not have speaking-active class
		"Carol Wrapped": true,  // has speaking-active class
	}

	if len(participants) != len(want) {
		t.Fatalf("expected %d participants, got %d: %+v", len(want), len(participants), participants)
	}
	for _, p := range participants {
		wantSpeaking, ok := want[p.Name]
		if !ok {
			t.Errorf("unexpected participant: %q", p.Name)
			continue
		}
		if p.Speaking != wantSpeaking {
			t.Errorf("participant %q: speaking = %v, want %v", p.Name, p.Speaking, wantSpeaking)
		}
	}
}

// TestSnapshotJS_SkipsShortAndLongNames verifies the name length filter.
func TestSnapshotJS_SkipsShortAndLongNames(t *testing.T) {
	requireNode(t)

	p := teams.New()
	js := p.SnapshotExpression()

	html := `<html><body>
		<div data-tid="AB"><div data-tid="voice-level-stream-outline" class="cls"></div></div>
		<div data-tid="` + longName(81) + `"><div data-tid="voice-level-stream-outline" class="cls"></div></div>
		<div data-tid="Valid Name"><div data-tid="voice-level-stream-outline" class="cls"></div></div>
	</body></html>`

	result := evalWithFixture(t, js, html)

	snapshots, err := p.ParseSnapshot(result)
	if err != nil {
		t.Fatalf("ParseSnapshot: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("expected 1 participant (length-filtered), got %d: %+v", len(snapshots), snapshots)
	}
	if snapshots[0].Name != "Valid Name" {
		t.Errorf("expected 'Valid Name', got %q", snapshots[0].Name)
	}
}

// teamsFixtureHTML replicates the two nesting patterns observed in live Teams meetings:
//   - "Direct": name data-tid on the immediate parent of voice-level-stream-outline
//   - "Nested": an extra wrapper div between voice-level-stream-outline and the named ancestor
//   - "Wrapped": video-item-container intermediate that should be skipped
const teamsFixtureHTML = `<html><body>
<div data-tid="MixedStage-wrapper">
  <div data-tid="calling-pagination">
    <div data-tid="only-videos-wrapper" role="menu">

      <!-- Pattern 1: name at parent (depth 1) -->
      <div role="menuitem">
        <div data-tid="video-item-container-Alice Direct">
          <div data-tid="Alice Direct">
            <div data-tid="voice-level-stream-outline" class="fui-Flex speaking-active f22iagw"></div>
          </div>
        </div>
      </div>

      <!-- Pattern 2: extra wrapper div (depth 2) -->
      <div role="menuitem">
        <div data-tid="video-item-container-Bob Nested">
          <div data-tid="Bob Nested">
            <div>
              <div data-tid="voice-level-stream-outline" class="fui-Flex f22iagw"></div>
            </div>
          </div>
        </div>
      </div>

      <!-- Pattern 3: video-item-container between voice-level and name -->
      <div role="menuitem">
        <div data-tid="video-item-container-Carol Wrapped">
          <div data-tid="Carol Wrapped">
            <div>
              <div data-tid="voice-level-stream-outline" class="fui-Flex speaking-active f22iagw"></div>
            </div>
          </div>
        </div>
      </div>

    </div>
  </div>
</div>
</body></html>`

func requireNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
}

func longName(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// evalWithFixture evaluates a JavaScript expression against a fixture HTML
// document using Node.js with a minimal DOM shim (linkedom-compatible subset).
func evalWithFixture(t *testing.T, jsExpr, html string) string {
	t.Helper()

	script := domShimScript(html, jsExpr)

	f, err := os.CreateTemp("", "teams-js-test-*.mjs")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	if _, err := f.WriteString(script); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "node", f.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node failed: %v\noutput: %s", err, out)
	}

	// Validate it's valid JSON
	var raw json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("output is not valid JSON: %s", out)
	}

	return string(out)
}

// domShimScript builds a self-contained Node.js script that parses HTML with a
// minimal DOM implementation and evaluates the given expression.
func domShimScript(html, jsExpr string) string {
	htmlJSON, _ := json.Marshal(html)
	exprJSON, _ := json.Marshal(jsExpr)

	return `
// Minimal DOM shim: implements only the subset used by Teams JS expressions.
class Element {
  constructor(tagName, attrs, parent) {
    this.tagName = tagName;
    this.attributes = new Map(Object.entries(attrs || {}));
    this.parentElement = parent || null;
    this.children = [];
    this.className = this.attributes.get('class') || '';
    const classSet = new Set(this.className.split(/\s+/).filter(Boolean));
    this.classList = { contains: (c) => classSet.has(c) };
  }
  getAttribute(name) { return this.attributes.get(name) || null; }
  querySelectorAll(selector) { return querySelectorAll(this, selector); }
}

function querySelectorAll(root, selector) {
  const match = selector.match(/^\[([^=]+)="([^"]+)"\]$/);
  if (!match) throw new Error('unsupported selector: ' + selector);
  const [, attr, value] = match;
  const results = [];
  function walk(el) {
    if (el.getAttribute(attr) === value) results.push(el);
    for (const child of el.children) walk(child);
  }
  walk(root);
  return results;
}

function parseHTML(html) {
  // Simple HTML parser: extracts nested div elements with their attributes.
  const root = new Element('BODY', {}, null);
  const stack = [root];

  const tagRe = /<\/?([a-z]+)([^>]*)>/gi;
  let m;
  while ((m = tagRe.exec(html)) !== null) {
    const [full, tag, attrStr] = m;
    if (full.startsWith('</')) {
      if (stack.length > 1) stack.pop();
      continue;
    }
    if (tag === '!--' || tag === 'html' || tag === 'head' || tag === 'meta' || tag === 'link') continue;

    const attrs = {};
    const attrRe = /([a-z][\w-]*)="([^"]*)"/gi;
    let am;
    while ((am = attrRe.exec(attrStr)) !== null) {
      attrs[am[1]] = am[2];
    }

    const parent = stack[stack.length - 1];
    const el = new Element(tag.toUpperCase(), attrs, parent);
    parent.children.push(el);

    // Self-closing tags
    if (!full.endsWith('/>') && !['br','hr','img','input'].includes(tag)) {
      stack.push(el);
    }
  }
  return root;
}

const html = ` + string(htmlJSON) + `;
const document = parseHTML(html);

// Make Array.from work on our arrays (it already does in Node)
const expr = ` + string(exprJSON) + `;
const result = eval(expr);
process.stdout.write(result);
`
}
