package speaker

type PlatformConfig struct {
	Name           string
	URLPattern     string
	SnapshotJS     string
	PollJSTemplate string
}

var MeetPlatform = PlatformConfig{
	Name:       "meet",
	URLPattern: "meet.google.com",
	SnapshotJS: `(function() {
  function getName(tile) {
    var names = Array.from(tile.querySelectorAll('.notranslate'))
      .map(function(n) { return n.innerText.trim(); })
      .filter(function(s) {
        return s.length > 2 && s.length < 50
          && s[0] === s[0].toUpperCase()
          && s.includes(' ');
      });
    return names[0] || null;
  }
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-participant-id]')).map(function(t) {
      var name = getName(t);
      var classes = new Set();
      t.querySelectorAll('[class]').forEach(function(el) {
        el.classList.forEach(function(c) { classes.add(c); });
      });
      return {name: name, classes: Array.from(classes)};
    }).filter(function(x) { return x.name; })
  );
})()`,
	PollJSTemplate: `(function() {
  function getName(tile) {
    var names = Array.from(tile.querySelectorAll('.notranslate'))
      .map(function(n) { return n.innerText.trim(); })
      .filter(function(s) {
        return s.length > 2 && s.length < 50
          && s[0] === s[0].toUpperCase()
          && s.includes(' ');
      });
    return names[0] || null;
  }
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-participant-id]')).map(function(t) {
      var name = getName(t);
      if (!name) return null;
      var speaking = !!t.querySelector('.%s') || !!t.closest('.%s');
      return {name: name, speaking: speaking};
    }).filter(Boolean)
  );
})()`,
}

var TeamsPlatform = PlatformConfig{
	Name:       "teams",
	URLPattern: "teams.microsoft.com",
	SnapshotJS: `(function() {
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-tid="voice-level-stream-outline"]')).map(function(el) {
      var p = el.parentElement;
      var tid = p ? p.getAttribute('data-tid') : null;
      var name = (tid && tid.length > 2 && tid.length < 80) ? tid : null;
      var classes = el.className.split(/\s+/);
      return {name: name, classes: classes};
    }).filter(function(x) { return x.name; })
  );
})()`,
	PollJSTemplate: `(function() {
  return JSON.stringify(
    Array.from(document.querySelectorAll('[data-tid="voice-level-stream-outline"]')).map(function(el) {
      var p = el.parentElement;
      var tid = p ? p.getAttribute('data-tid') : null;
      if (!tid || tid.length <= 2) return null;
      var speaking = el.classList.contains('%s');
      return {name: tid, speaking: speaking};
    }).filter(Boolean)
  );
})()`,
}

var Platforms = []PlatformConfig{MeetPlatform, TeamsPlatform}
